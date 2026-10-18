# Working on yawntest

Read CONTRIBUTING.md first: setup, golden files, adding a rule, the real-world corpus.

## Code

- Standard library only. No new dependencies.
- Comments only where they're needed: why something is done, a non-obvious constraint, an
  example that makes a rule clear. Not what the next line does.
- Behavior changes come with a fixture or test that fails without them. Check that it does.
- Changed output shows up in a golden diff; read it before committing.

## Before every push

```bash
gofmt -l .                        # must print nothing
go vet ./...
go test -race ./...
GOTOOLCHAIN=go1.23.0 go test ./...  # the oldest Go that go.mod promises
go run ./cmd/yawntest check ./...   # yawntest checks itself
```

Run each on its own, so one failing doesn't hide the others.

## Commits and pull requests

- Several small pull requests (under ~500 lines) instead of one big one. Stacked pull requests
  are kept in sync with merge commits, never rebases or force pushes.
- Commit messages end with `Co-Authored-By: Claude <noreply@anthropic.com>` and nothing else:
  no session links, no model names.
- Pull request descriptions end with "Made with Claude", again without session links.
- CHANGELOG.md gets an entry under Unreleased; README and docs/how-it-works.md follow
  behavior changes.
- Review findings (CodeRabbit or people) are checked against the code before they're fixed.
  Answer each thread: fixed in which commit, or why not.
