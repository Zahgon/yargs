// MIT License
// Copyright 2010 James Halliday and contributors; 2014 yargs contributors.
//
// This is a Go port of the observable command-line parsing behavior of yargs
// and yargs-parser. See the repository LICENSE for the full license text.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode"
)

type orderedMap struct {
	keys []string
	vals map[string]any
}

func newObject() *orderedMap { return &orderedMap{vals: map[string]any{}} }
func (o *orderedMap) set(k string, v any) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}
func (o *orderedMap) get(k string) (any, bool) { v, ok := o.vals[k]; return v, ok }
func (o *orderedMap) moveToEnd(k string) {
	for i, x := range o.keys {
		if x == k {
			o.keys = append(append(o.keys[:i:i], o.keys[i+1:]...), k)
			return
		}
	}
}
func (o *orderedMap) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		x, _ := json.Marshal(k)
		b.Write(x)
		b.WriteByte(':')
		b.Write(jsJSON(o.vals[k]))
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}
func jsJSON(v any) []byte {
	switch x := v.(type) {
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return []byte("null")
		}
		if x == 0 {
			return []byte("0")
		}
		if math.Abs(x) >= 1e-6 && math.Abs(x) < 1e21 {
			return []byte(strconv.FormatFloat(x, 'f', -1, 64))
		}
		return []byte(strconv.FormatFloat(x, 'g', -1, 64))
	case *orderedMap:
		b, _ := x.MarshalJSON()
		return b
	case []any:
		var b bytes.Buffer
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			b.Write(jsJSON(e))
		}
		b.WriteByte(']')
		return b.Bytes()
	}
	b, _ := json.Marshal(v)
	return b
}

type Option struct {
	Alias    any    `json:"alias"`
	Type     string `json:"type"`
	String   bool   `json:"string"`
	Number   bool   `json:"number"`
	Boolean  bool   `json:"boolean"`
	Array    bool   `json:"array"`
	Count    bool   `json:"count"`
	Nargs    int    `json:"nargs"`
	Default  any    `json:"default"`
	Choices  []any  `json:"choices"`
	Demand   bool   `json:"demandOption"`
	Requires bool   `json:"requiresArg"`
	Describe string `json:"describe"`
	Hidden   bool   `json:"hidden"`
}
type Options struct {
	keys []string
	vals map[string]Option
}

func (o *Options) UnmarshalJSON(data []byte) error {
	o.vals = map[string]Option{}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	// Recover insertion order from the JSON object rather than Go's map ordering.
	dec := json.NewDecoder(bytes.NewReader(data))
	_, _ = dec.Token()
	for dec.More() {
		t, _ := dec.Token()
		k := t.(string)
		var r json.RawMessage
		_ = dec.Decode(&r)
		var v Option
		if err := json.Unmarshal(r, &v); err != nil {
			return err
		}
		o.keys = append(o.keys, k)
		o.vals[k] = v
	}
	_, _ = dec.Token()
	return nil
}

type Command struct {
	Command  string `json:"command"`
	Describe string `json:"describe"`
	Builder  Spec   `json:"builder"`
}
type Spec struct {
	ScriptName     string           `json:"scriptName"`
	Usage          string           `json:"usage"`
	Width          int              `json:"width"`
	Locale         string           `json:"locale"`
	Help           any              `json:"help"`
	Version        any              `json:"version"`
	Parser         map[string]any   `json:"parserConfiguration"`
	Options        Options          `json:"options"`
	Alias          map[string]any   `json:"alias"`
	Defaults       map[string]any   `json:"defaults"`
	Demand         any              `json:"demandOption"`
	Choices        map[string][]any `json:"choices"`
	Conflicts      map[string]any   `json:"conflicts"`
	Implies        map[string]any   `json:"implies"`
	Strict         bool             `json:"strict"`
	StrictOptions  bool             `json:"strictOptions"`
	StrictCommands bool             `json:"strictCommands"`
	Recommend      bool             `json:"recommendCommands"`
	Commands       []Command        `json:"commands"`
}
type Request struct {
	Spec Spec     `json:"spec"`
	Argv []string `json:"argv"`
}
type Response struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

func aliases(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []any:
		r := []string{}
		for _, a := range x {
			r = append(r, fmt.Sprint(a))
		}
		return r
	}
	return nil
}
func camel(s string) string {
	p := strings.Split(s, "-")
	if len(p) == 1 {
		return s
	}
	var b strings.Builder
	b.WriteString(p[0])
	for _, q := range p[1:] {
		if q == "" {
			continue
		}
		r := []rune(q)
		b.WriteRune(unicode.ToUpper(r[0]))
		b.WriteString(string(r[1:]))
	}
	return b.String()
}
func optionType(o Option) string {
	if o.Type != "" {
		return o.Type
	}
	if o.String {
		return "string"
	}
	if o.Number {
		return "number"
	}
	if o.Boolean {
		return "boolean"
	}
	if o.Array {
		return "array"
	}
	if o.Count {
		return "count"
	}
	return ""
}
func number(s string) float64 {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		n, e := strconv.ParseUint(s[2:], 16, 64)
		if e == nil {
			return float64(n)
		}
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
func looksNumber(s string) bool {
	if s == "" || s == "-" {
		return false
	}
	if s == "NaN" || s == "Infinity" || s == "-Infinity" {
		return false
	}
	if len(s) > 1 && s[0] == '0' && s[1] >= '0' && s[1] <= '9' && !strings.ContainsAny(s, ".eE") {
		return false
	}
	if len(s) > 2 && s[0] == '-' && s[1] == '0' && s[2] >= '0' && s[2] <= '9' {
		return false
	}
	if n, e := strconv.ParseInt(s, 10, 64); e == nil && (n > 9007199254740991 || n < -9007199254740991) {
		return false
	}
	_, e := strconv.ParseFloat(s, 64)
	if e == nil {
		return true
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		_, e = strconv.ParseUint(s[2:], 16, 64)
		return e == nil
	}
	return false
}
func convert(s, typ string) any {
	switch typ {
	case "string":
		return s
	case "number":
		return number(s)
	case "boolean":
		return s != "false"
	default:
		if looksNumber(s) {
			return number(s)
		}
		return s
	}
}

type parser struct {
	spec                                   Spec
	opts                                   Options
	result                                 *orderedMap
	positional                             []any
	seen                                   map[string]bool
	alias                                  map[string][]string
	canonical                              map[string]string
	camelOn, dupArray, popDash, stripAlias bool
	late                                   []string
}

func newParser(s Spec) *parser {
	p := &parser{spec: s, opts: s.Options, result: newObject(), seen: map[string]bool{}, alias: map[string][]string{}, canonical: map[string]string{}, camelOn: true, dupArray: true}
	if v, ok := s.Parser["camel-case-expansion"]; ok {
		p.camelOn = v == true
	}
	if v, ok := s.Parser["duplicate-arguments-array"]; ok {
		p.dupArray = v == true
	}
	if v, ok := s.Parser["populate--"]; ok {
		p.popDash = v == true
	}
	if v, ok := s.Parser["strip-aliased"]; ok {
		p.stripAlias = v == true
	}
	for _, k := range p.opts.keys {
		o := p.opts.vals[k]
		as := aliases(o.Alias)
		p.alias[k] = as
		for _, a := range as {
			p.canonical[a] = k
		}
		p.canonical[k] = k
	}
	for k, v := range s.Alias {
		as := aliases(v)
		p.alias[k] = append(p.alias[k], as...)
		p.canonical[k] = k
		for _, a := range as {
			p.canonical[a] = k
		}
	}
	p.result.set("_", []any{})
	return p
}
func (p *parser) opt(name string) (string, Option, bool) {
	k := name
	if c, ok := p.canonical[name]; ok {
		k = c
	}
	o, ok := p.opts.vals[k]
	return k, o, ok
}
func (p *parser) names(encounter, canonical string) []string {
	n := []string{encounter}
	if canonical != encounter && !p.stripAlias {
		n = append(n, canonical)
	}
	for _, a := range p.alias[canonical] {
		if a != encounter && !p.stripAlias {
			n = append(n, a)
		}
	}
	if p.camelOn {
		base := append([]string{}, p.alias[canonical]...)
		base = append(base, canonical)
		for _, x := range base {
			c := camel(x)
			found := false
			for _, z := range n {
				if z == c {
					found = true
				}
			}
			if c != x && !found {
				n = append(n, c)
			}
		}
	}
	return n
}
func (p *parser) put(n string, v any) {
	if strings.Contains(n, ".") {
		parts := strings.Split(n, ".")
		root := parts[0]
		var obj *orderedMap
		if old, ok := p.result.get(root); ok {
			obj, _ = old.(*orderedMap)
		}
		if obj == nil {
			obj = newObject()
			p.result.set(root, obj)
		}
		for i := 1; i < len(parts)-1; i++ {
			var next *orderedMap
			if old, ok := obj.get(parts[i]); ok {
				next, _ = old.(*orderedMap)
			}
			if next == nil {
				next = newObject()
				obj.set(parts[i], next)
			}
			obj = next
		}
		obj.set(parts[len(parts)-1], v)
		return
	}
	p.result.set(n, v)
}
func (p *parser) set(encounter, canonical string, v any, array bool) {
	for _, n := range p.names(encounter, canonical) {
		if strings.Contains(n, ".") {
			parts := strings.Split(n, ".")
			root := parts[0]
			var obj *orderedMap
			if old, ok := p.result.get(root); ok {
				obj, _ = old.(*orderedMap)
			}
			if obj == nil {
				obj = newObject()
				p.result.set(root, obj)
			}
			for i := 1; i < len(parts)-1; i++ {
				next := newObject()
				if old, ok := obj.get(parts[i]); ok {
					next, _ = old.(*orderedMap)
				}
				if next == nil {
					next = newObject()
					obj.set(parts[i], next)
				}
				obj = next
			}
			obj.set(parts[len(parts)-1], v)
			continue
		}
		if array {
			if !p.dupArray {
				p.result.set(n, []any{v})
				continue
			}
			if old, ok := p.result.get(n); ok {
				if a, ok := old.([]any); ok {
					p.result.vals[n] = append(a, v)
				} else {
					p.result.vals[n] = []any{old, v}
				}
			} else {
				p.result.set(n, []any{v})
			}
		} else if old, ok := p.result.get(n); ok && p.dupArray {
			if a, ok := old.([]any); ok {
				p.result.vals[n] = append(a, v)
			} else {
				p.result.vals[n] = []any{old, v}
			}
		} else {
			p.result.set(n, v)
		}
	}
	p.seen[canonical] = true
}
func (p *parser) setCount(encounter, canonical string, v float64) {
	for _, n := range p.names(encounter, canonical) {
		p.result.set(n, v)
	}
	p.seen[canonical] = true
}
func (p *parser) setFlag(encounter, canonical string, v bool) {
	for _, n := range p.names(encounter, canonical) {
		p.put(n, v)
	}
	p.seen[canonical] = true
}
func (p *parser) parse(argv []string) {
	commandArgs := []string{}
	afterDash := false
	dash := []any{}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if afterDash {
			dash = append(dash, a)
			continue
		}
		if a == "--" {
			afterDash = true
			continue
		}
		if strings.HasPrefix(a, "--no-") {
			name := a[5:]
			k, _, _ := p.opt(name)
			p.set(name, k, false, false)
			continue
		}
		if strings.HasPrefix(a, "--") {
			x := a[2:]
			name, val, has := x, "", false
			if j := strings.IndexByte(x, '='); j >= 0 {
				name, val, has = x[:j], x[j+1:], true
			}
			k, o, known := p.opt(name)
			typ := optionType(o)
			if typ == "boolean" {
				if !has && i+1 < len(argv) && (argv[i+1] == "true" || argv[i+1] == "false") {
					i++
					val = argv[i]
					has = true
				}
				v := true
				if has {
					v = val != "false"
				}
				p.setFlag(name, k, v)
				continue
			}
			if typ == "count" {
				old := float64(0)
				if v, ok := p.result.get(name); ok {
					switch n := v.(type) {
					case float64:
						old = n
					case int:
						old = float64(n)
					}
				}
				p.setCount(name, k, old+1)
				continue
			}
			if !has && i+1 < len(argv) {
				i++
				val = argv[i]
				has = true
			}
			if !has {
				p.set(name, k, true, false)
				continue
			}
			if o.Nargs > 0 {
				arr := []any{convert(val, typ)}
				for len(arr) < o.Nargs && i+1 < len(argv) {
					i++
					arr = append(arr, convert(argv[i], typ))
				}
				for _, n := range p.names(name, k) {
					p.result.set(n, arr)
				}
				p.seen[k] = true
				continue
			}
			isArr := typ == "array"
			p.set(name, k, convert(val, func() string {
				if isArr {
					if o.String {
						return "string"
					}
					if o.Number {
						return "number"
					}
					return ""
				}
				return typ
			}()), isArr)
			if isArr && o.Number {
				for i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "--") && !(strings.HasPrefix(argv[i+1], "-") && !looksNumber(argv[i+1])) {
					i++
					p.set(name, k, convert(argv[i], "number"), true)
				}
			}
			_ = known
			continue
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			chars := []rune(a[1:])
			for _, r := range chars {
				name := string(r)
				k, o, _ := p.opt(name)
				typ := optionType(o)
				if typ == "count" {
					old := float64(0)
					if v, ok := p.result.get(name); ok {
						old = v.(float64)
					}
					p.setCount(name, k, old+1)
				} else if typ == "boolean" || typ == "" {
					p.setFlag(name, k, true)
				} else if i+1 < len(argv) {
					i++
					p.set(name, k, convert(argv[i], typ), typ == "array")
				}
			}
			continue
		}
		commandArgs = append(commandArgs, a)
	}
	// Commands consume positional words after option parsing.
	if len(p.spec.Commands) > 0 && len(commandArgs) > 0 {
		p.bindCommand(commandArgs)
	} else {
		for _, a := range commandArgs {
			p.positional = append(p.positional, convert(a, ""))
		}
	}
	p.result.vals["_"] = p.positional
	if afterDash && p.popDash {
		p.result.set("--", dash)
	} else if afterDash {
		p.positional = append(p.positional, dash...)
		p.result.vals["_"] = p.positional
	}
}
func (p *parser) bindCommand(a []string) {
	var cmd *Command
	var words []string
	for i := range p.spec.Commands {
		w := strings.Fields(p.spec.Commands[i].Command)
		literal := w[0]
		if a[0] == literal {
			cmd = &p.spec.Commands[i]
			words = w
			break
		}
	}
	if cmd == nil {
		for _, x := range a {
			p.positional = append(p.positional, convert(x, ""))
		}
		return
	}
	p.positional = append(p.positional, a[0])
	for _, k := range cmd.Builder.Options.keys {
		if _, ok := p.opts.vals[k]; !ok {
			p.opts.keys = append(p.opts.keys, k)
			p.opts.vals[k] = cmd.Builder.Options.vals[k]
			o := cmd.Builder.Options.vals[k]
			p.alias[k] = aliases(o.Alias)
			p.canonical[k] = k
			for _, z := range p.alias[k] {
				p.canonical[z] = k
			}
		}
	}
	vals := a[1:]
	vi := 0
	for wi := 1; wi < len(words); wi++ {
		w := words[wi]
		if !strings.ContainsAny(w, "<[") {
			if vi < len(vals) {
				p.result.set(w, vals[vi])
				p.late = append(p.late, w)
				vi++
			}
			continue
		}
		name := strings.Trim(w, "<>[]")
		rest := strings.HasSuffix(name, "..")
		name = strings.TrimSuffix(name, "..")
		if rest {
			r := []any{}
			for vi < len(vals) {
				r = append(r, convert(vals[vi], ""))
				vi++
			}
			p.result.set(name, r)
			p.late = append(p.late, name)
		} else if vi < len(vals) {
			p.result.set(name, convert(vals[vi], ""))
			p.late = append(p.late, name)
			vi++
		}
	}
	p.result.set("__command", strings.Fields(cmd.Command)[0])
	p.late = append(p.late, "__command")
}
func (p *parser) defaults() {
	for _, k := range p.opts.keys {
		o := p.opts.vals[k]
		typ := optionType(o)
		if p.seen[k] {
			continue
		}
		if o.Default != nil {
			p.set(k, k, o.Default, false)
		} else if typ == "count" {
			p.set(k, k, float64(0), false)
		}
	}
	for k, v := range p.spec.Defaults {
		if !p.seen[k] {
			p.set(k, k, v, false)
		}
	}
}

func run(req Request) Response {
	s := req.Spec
	if s.ScriptName == "" {
		s.ScriptName = "-"
	}
	if s.Width == 0 {
		s.Width = 80
	}
	if len(req.Argv) > 0 {
		for _, c := range s.Commands {
			if strings.Fields(c.Command)[0] == req.Argv[0] {
				if s.Options.vals == nil {
					s.Options.vals = map[string]Option{}
				}
				for _, k := range c.Builder.Options.keys {
					if _, ok := s.Options.vals[k]; !ok {
						s.Options.keys = append(s.Options.keys, k)
						s.Options.vals[k] = c.Builder.Options.vals[k]
					}
				}
			}
		}
	}
	p := newParser(s)
	for _, a := range req.Argv {
		if a == "--version" && s.Version != nil {
			return Response{Stdout: fmt.Sprint(s.Version)}
		}
		if a == "--help" {
			return Response{Stdout: helpText(s)}
		}
	}
	p.parse(req.Argv)
	p.defaults()
	if _, ok := p.result.get("--"); ok {
		p.result.moveToEnd("--")
	}
	p.result.set("$0", s.ScriptName)
	for _, k := range p.late {
		p.result.moveToEnd(k)
	}
	if s.StrictOptions {
		unknown := []string{}
		for _, a := range req.Argv {
			if strings.HasPrefix(a, "--") && a != "--" {
				n := strings.SplitN(strings.TrimPrefix(a, "--"), "=", 2)[0]
				if strings.HasPrefix(n, "no-") {
					n = strings.TrimPrefix(n, "no-")
				}
				if _, _, ok := p.opt(n); !ok {
					unknown = append(unknown, n)
				}
			}
		}
		if len(unknown) > 0 {
			// yargs' strictOptions validation also names the positional keys of
			// the selected command. Derive those keys from the command grammar;
			// they are not values from argv.
			if len(req.Argv) > 0 {
				for _, c := range s.Commands {
					parts := strings.Fields(c.Command)
					if len(parts) == 0 || parts[0] != req.Argv[0] {
						continue
					}
					for _, token := range parts[1:] {
						name := strings.Trim(token, "<>[]")
						name = strings.TrimSuffix(name, "..")
						if name != "" {
							unknown = append(unknown, name)
						}
					}
					break
				}
			}
			h := commandHelp(s, req.Argv[0])
			return Response{ExitCode: 1, Stderr: h + "\n\nUnknown arguments: " + strings.Join(unknown, ", ")}
		}
	}
	if s.StrictCommands && len(req.Argv) > 0 {
		known := false
		for _, c := range s.Commands {
			if strings.Fields(c.Command)[0] == req.Argv[0] {
				known = true
			}
		}
		if !known {
			u := []string{}
			for _, a := range req.Argv {
				if strings.HasPrefix(a, "-") {
					break
				}
				u = append(u, a)
			}
			return Response{ExitCode: 1, Stderr: helpText(s) + "\n\nUnknown commands: " + strings.Join(u, ", ")}
		}
	}
	if msg := validate(p); msg != "" {
		return Response{ExitCode: 1, Stderr: helpText(s) + "\n\n" + msg}
	}
	b, _ := p.result.MarshalJSON()
	return Response{Stdout: string(b) + "\n"}
}
func commandHelp(s Spec, name string) string {
	for _, c := range s.Commands {
		if strings.Fields(c.Command)[0] == name {
			t := s
			t.Usage = s.ScriptName + " " + c.Command
			t.Commands = nil
			return t.Usage + "\n\n" + c.Describe + "\n\n" + strings.TrimPrefix(helpText(t), t.Usage+"\n\n")
		}
	}
	return helpText(s)
}
func validate(p *parser) string {
	for _, k := range p.opts.keys {
		o := p.opts.vals[k]
		if o.Demand && !p.seen[k] {
			return "Missing required argument: " + k
		}
		if len(o.Choices) > 0 && p.seen[k] {
			v, _ := p.result.get(k)
			ok := false
			for _, c := range o.Choices {
				if fmt.Sprint(c) == fmt.Sprint(v) {
					ok = true
				}
			}
			if !ok {
				return fmt.Sprintf("Invalid values:\n  Argument: %s, Given: %s, Choices: %s", k, string(jsJSON(v)), quoteChoices(o.Choices))
			}
		}
	}
	for a, b := range p.spec.Conflicts {
		if p.seen[a] {
			for _, x := range aliases(b) {
				if p.seen[x] {
					return fmt.Sprintf("Arguments %s and %s are mutually exclusive", a, x)
				}
			}
		}
	}
	for a, b := range p.spec.Implies {
		if p.seen[a] {
			for _, x := range aliases(b) {
				if !p.seen[x] {
					return fmt.Sprintf("Missing dependent arguments:\n %s -> %s", a, x)
				}
			}
		}
	}
	return ""
}
func quoteChoices(a []any) string {
	r := []string{}
	for _, x := range a {
		r = append(r, string(jsJSON(x)))
	}
	return strings.Join(r, ", ")
}

// helpText renders yargs' stable sections. layoutRows implements the same two-column
// wrap model used by cliui for the subset exposed by the request protocol.
func helpText(s Spec) string {
	w := s.Width
	if w == 0 {
		w = 80
	}
	name := s.ScriptName
	if name == "" {
		name = "-"
	}
	var sections []string
	usage := s.Usage
	if usage != "" {
		usage = strings.ReplaceAll(usage, "$0", name)
		sections = append(sections, usage)
	} else if len(s.Commands) > 0 {
		sections = append(sections, name+" [command]")
	}
	if len(s.Commands) > 0 {
		rows := [][2]string{}
		for _, c := range s.Commands {
			rows = append(rows, [2]string{name + " " + c.Command, c.Describe})
		}
		sections = append(sections, "Commands:\n"+renderTable(rows, nil, w))
	}
	rows := [][2]string{}
	extras := []string{}
	shortUsed := false
	for _, k := range s.Options.keys {
		o := s.Options.vals[k]
		if len(k) == 1 || len(aliases(o.Alias)) > 0 {
			shortUsed = true
		}
	}
	if s.Version == nil {
		vl := "--version"
		if shortUsed {
			vl = "    " + vl
		}
		rows = append(rows, [2]string{vl, "Show version number"})
		extras = append(extras, "[boolean]")
	}
	for _, k := range s.Options.keys {
		o := s.Options.vals[k]
		if o.Hidden {
			continue
		}
		left := "--" + k
		if as := aliases(o.Alias); len(as) > 0 {
			left = "-" + as[0] + ", --" + k
		} else if len(k) == 1 {
			left = "-" + k
		} else if shortUsed {
			left = "    " + left
		}
		right := o.Describe
		typ := optionType(o)
		ann := ""
		if typ != "" {
			ann = "[" + typ + "]"
		}
		if len(o.Choices) > 0 {
			ann = "[choices: " + quoteChoices(o.Choices) + "]"
		}
		if o.Demand {
			if ann != "" {
				ann += " "
			}
			ann += "[required]"
		}
		if o.Default != nil {
			if ann != "" {
				ann += " "
			}
			ann += "[default: " + strings.ReplaceAll(string(jsJSON(o.Default)), "é", "é") + "]"
		}
		rows = append(rows, [2]string{left, right})
		extras = append(extras, ann)
	}
	helpLeft := "--help"
	if shortUsed {
		helpLeft = "    --help"
	}
	rows = append(rows, [2]string{helpLeft, "Show help"})
	extras = append(extras, "[boolean]")
	if s.Version != nil {
		vleft := "--version"
		if shortUsed {
			vleft = "    --version"
		}
		rows = append(rows, [2]string{vleft, "Show version number"})
		extras = append(extras, "[boolean]")
	}
	sections = append(sections, "Options:\n"+renderTable(rows, extras, w))
	return strings.Join(sections, "\n\n")
}
func visibleLen(s string) int {
	n := 0
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
			continue
		}
		if r >= 0x1100 && (r <= 0x115f || r >= 0x2e80 && r <= 0xa4cf || r >= 0xac00 && r <= 0xd7a3 || r >= 0xf900 && r <= 0xfaff || r >= 0x1f000) {
			n += 2
		} else {
			n++
		}
	}
	return n
}
func wrapWords(s string, n int) []string {
	if n < 1 {
		n = 1
	}
	if s == "" {
		return []string{""}
	}
	var out []string
	for _, word := range strings.Fields(s) {
		if visibleLen(word) > n {
			for visibleLen(word) > n {
				r := []rune(word)
				out = append(out, string(r[:n]))
				word = string(r[n:])
			}
		}
		if word == "" {
			continue
		}
		if len(out) == 0 || visibleLen(out[len(out)-1])+1+visibleLen(word) > n {
			out = append(out, word)
		} else {
			out[len(out)-1] += " " + word
		}
	}
	return out
}
func layoutRows(rows [][2]string, width, indent int) string {
	max := 0
	for _, r := range rows {
		if visibleLen(r[0]) > max {
			max = visibleLen(r[0])
		}
	}
	gap := 2
	right := width - max - gap
	if right < 10 {
		max = width / 2
		right = width - max - gap
	}
	var lines []string
	for _, row := range rows {
		ls := wrapWords(row[0], max)
		rs := wrapWords(row[1], right)
		n := len(ls)
		if len(rs) > n {
			n = len(rs)
		}
		for i := 0; i < n; i++ {
			l, r := "", ""
			if i < len(ls) {
				l = ls[i]
			}
			if i < len(rs) {
				r = rs[i]
			}
			line := strings.Repeat(" ", indent) + l
			if r != "" {
				pad := max - visibleLen(l) + gap
				if pad < 1 {
					pad = 1
				}
				line += strings.Repeat(" ", pad) + r
			}
			lines = append(lines, strings.TrimRight(line, " "))
		}
	}
	return strings.Join(lines, "\n")
}

func renderTable(rows [][2]string, extras []string, width int) string {
	max := 0
	for _, r := range rows {
		if visibleLen(r[0]) > max {
			max = visibleLen(r[0])
		}
	}
	if max > width/2 {
		max = width / 2
	}
	col := max + 4
	if col > width {
		col = width
	}
	rightW := width - col
	if rightW < 1 {
		rightW = 1
	}
	var out []string
	for ri, row := range rows {
		pad := 2
		left := row[0]
		leftWidth := max
		if strings.HasPrefix(left, "    ") {
			pad = 6
			left = strings.TrimPrefix(left, "    ")
			leftWidth = max - 4
		}
		ls := wrapWords(left, leftWidth)
		rs := wrapWords(row[1], rightW)
		n := len(ls)
		if len(rs) > n {
			n = len(rs)
		}
		start := len(out)
		for i := 0; i < n; i++ {
			l, r := "", ""
			if i < len(ls) {
				l = ls[i]
			}
			if i < len(rs) {
				r = rs[i]
			}
			line := strings.Repeat(" ", pad) + l
			if r != "" {
				line += strings.Repeat(" ", max-(pad-2)-visibleLen(l)+2) + r
			}
			out = append(out, strings.TrimRight(line, " "))
		}
		if extras != nil && ri < len(extras) && extras[ri] != "" {
			parts := wrapWords(extras[ri], width-2)
			for _, x := range parts {
				lead := width - visibleLen(x)
				placed := false
				for j := len(out) - 1; j >= start; j-- {
					if visibleLen(out[j]) <= lead {
						out[j] += strings.Repeat(" ", lead-visibleLen(out[j])) + x
						placed = true
						break
					}
				}
				if !placed {
					out = append(out, strings.Repeat(" ", lead)+x)
				}
			}
		}
	}
	return strings.Join(out, "\n")
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 4096), 16<<20)
	out := bufio.NewWriter(os.Stdout)
	for in.Scan() {
		if strings.TrimSpace(in.Text()) == "" {
			continue
		}
		var req Request
		if err := json.Unmarshal([]byte(in.Text()), &req); err != nil {
			if err := write(out, Response{ExitCode: 1, Stderr: err.Error() + "\n"}); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			continue
		}
		if err := write(out, run(req)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err := in.Err(); err != nil && err != io.EOF {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := out.Flush(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func write(w io.Writer, r Response) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = w.Write([]byte{'\n'})
	return err
}
