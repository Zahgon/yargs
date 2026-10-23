# Contributing to Yargs

For this Go port, run `gofmt -w *.go`, `go test ./...`, and
`go build -o yargs .` before submitting a change. The module is deliberately
standard-library-only and must build offline. Behavioral changes need focused
Go tests and comparison with the pinned yargs 18.2.0 reference command.

Me hearties, we would love to have you contribute to yargs!

If you are looking to update the website, check out [yargs/yargs.github.io](https://github.com/yargs/yargs.github.io).

## Contributing

1. Look through the existing issues and see if your idea is something new.
2. Create a new issue, or comment on an existing issue that you would like
   to help solve:
    * it's usually best to get some feedback before proceeding to write code.
3. fork the yargs repo, and clone it to your computer:
    * GitHub has [great documentation](https://help.github.com/articles/using-pull-requests/) regarding writing your first pull request.
4. make sure that you write unit-test for any code that you write for yargs:
    * we use the [standard](https://github.com/feross/standard) coding style,
      which will validate your style when you run tests.
    * look through our extensive test suite in `/test` to get an idea for how
      to write unit-tests for this codebase.
5. make sure you are comfortable with the Code of Conduct outlined below.
