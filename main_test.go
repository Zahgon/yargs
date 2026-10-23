// MIT License. Copyright 2014 yargs contributors.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func request(t *testing.T, src string) Request {
	t.Helper()
	var r Request
	if err := json.Unmarshal([]byte(src), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestTypesAliasesAndDefaults(t *testing.T) {
	r := request(t, `{"spec":{"scriptName":"tool","options":{"port":{"type":"number","alias":"p"},"dry-run":{"type":"boolean"},"name":{"type":"string","default":"x"}}},"argv":["-p","08","--dry-run"]}`)
	got := run(r)
	want := `{"_":[],"p":8,"port":8,"dry-run":true,"dryRun":true,"name":"x","$0":"tool"}` + "\n"
	if got.ExitCode != 0 || got.Stdout != want {
		t.Fatalf("got %#v, want stdout %q", got, want)
	}
}

func TestArraysDotNotationAndSeparator(t *testing.T) {
	r := request(t, `{"spec":{"scriptName":"tool","parserConfiguration":{"populate--":true},"options":{"tag":{"type":"array","string":true},"db.port":{"type":"number"}}},"argv":["--tag","01","--tag","雪","--db.port","0x10","--","-x"]}`)
	got := run(r)
	for _, part := range []string{`"tag":["01","雪"]`, `"db":{"port":16}`, `"--":["-x"]`} {
		if !strings.Contains(got.Stdout, part) {
			t.Fatalf("%q lacks %q", got.Stdout, part)
		}
	}
}

func TestValidationAndHelp(t *testing.T) {
	r := request(t, `{"spec":{"scriptName":"tool","width":52,"options":{"mode":{"choices":["dev","prod"],"demandOption":true}}},"argv":[]}`)
	got := run(r)
	if got.ExitCode != 1 || !strings.HasSuffix(got.Stderr, "Missing required argument: mode") || !strings.Contains(got.Stderr, "[required]") {
		t.Fatalf("unexpected response: %#v", got)
	}
}

func TestCommandPositionals(t *testing.T) {
	r := request(t, `{"spec":{"scriptName":"tool","commands":[{"command":"serve <file> [rest..]","describe":"serve files","builder":{"options":{"port":{"type":"number","default":8080}}}}]},"argv":["serve","a","b","2"]}`)
	got := run(r)
	if !strings.Contains(got.Stdout, `"file":"a","rest":["b",2],"__command":"serve"`) {
		t.Fatal(got.Stdout)
	}
}

func TestCountsNegationAndNargs(t *testing.T) {
	r := request(t, `{"spec":{"options":{"verbose":{"type":"count"},"cache":{"type":"boolean","default":true},"pair":{"type":"array","nargs":2}}},"argv":["--verbose","--verbose","--no-cache","--pair","a","2"]}`)
	got := run(r)
	for _, part := range []string{`"verbose":2`, `"cache":false`, `"pair":["a",2]`} {
		if !strings.Contains(got.Stdout, part) {
			t.Fatalf("%q lacks %q", got.Stdout, part)
		}
	}
}

func TestChoicesConflictsAndImplies(t *testing.T) {
	tests := []string{
		`{"spec":{"options":{"mode":{"choices":["dev","prod"]}}},"argv":["--mode","bad"]}`,
		`{"spec":{"options":{"a":{"type":"boolean"},"b":{"type":"boolean"}},"conflicts":{"a":"b"}},"argv":["--a","--b"]}`,
		`{"spec":{"options":{"a":{"type":"boolean"},"b":{"type":"boolean"}},"implies":{"a":"b"}},"argv":["--a"]}`,
	}
	for _, src := range tests {
		if got := run(request(t, src)); got.ExitCode != 1 || got.Stderr == "" {
			t.Fatalf("expected validation failure, got %#v", got)
		}
	}
}

func TestStrictModesAndVersion(t *testing.T) {
	strict := request(t, `{"spec":{"strictOptions":true,"commands":[{"command":"run <target>"}]},"argv":["run","file","--unknown"]}`)
	got := run(strict)
	if got.ExitCode != 1 || !strings.Contains(got.Stderr, "Unknown arguments: unknown, target") {
		t.Fatalf("unexpected strict response: %#v", got)
	}
	version := request(t, `{"spec":{"version":"18.2.0"},"argv":["--version"]}`)
	if got := run(version); got.ExitCode != 0 || got.Stdout != "18.2.0" {
		t.Fatalf("unexpected version response: %#v", got)
	}
}
