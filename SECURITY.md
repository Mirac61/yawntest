# Security

yawntest runs on your machine and in your CI with your permissions: it reads your code, runs
`go list` and `go test` on it, and `gen` and `run` write `*_yawntest_test.go` files next to it.
yawntest itself sends nothing anywhere. The go tool it runs may download modules that aren't in
your module cache yet, through your configured `GOPROXY`, as any `go test` does. The generated
tests run your code, the same way your own tests do.

If you find a way yawntest could be made to do more than that (write outside the scanned
tree, run something other than the go tool and git, or be tricked by a malicious repository
into doing so), please don't open a public issue. Report it privately through
[GitHub's security advisories](https://github.com/Mirac61/yawntest/security/advisories/new)
instead. You'll get an answer within a week.

Only the latest release gets fixes.
