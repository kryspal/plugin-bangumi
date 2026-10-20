# Contributing

Thank you for helping with this plugin. Issues and pull requests are welcome.

## Sign your commits (DCO)

We use the [Developer Certificate of Origin](https://developercertificate.org/) (DCO) instead of a contributor license
agreement. By adding a `Signed-off-by` line to a commit you certify that you wrote the change or otherwise have the
right to submit it under the project's license:

```sh
git commit -s -m "Describe the change"
```

The line must carry your real name and an email address you can be reached at. Pull requests with unsigned commits
cannot be merged.

Contributions are licensed under the Apache License 2.0, as section 5 of the license describes; there is no separate
agreement to sign.

## What we can accept

- Your own work, or code under a license compatible with Apache 2.0 (for example MIT, BSD or Apache 2.0) with its
  copyright notice and attribution kept.
- No code copied from GPL, LGPL, AGPL or MPL projects, and no code of unknown origin.
- No new dependencies under copyleft licenses.
- Matching rules only for naming patterns that recur in real libraries, with a test case for each; the goal is a
  plugin that behaves predictably, not one tuned to a sample.
- Test data uses fictional names or the public titles of published works; never paths or names from a private library.

## Before you open a pull request

- `gofmt`, `go vet ./...` and `go test ./...` pass.
- New behavior has tests; user-visible changes are listed in `CHANGELOG.md`; `docs/design.md` reflects the rules.
- Documentation, code comments and commit messages are in English. String literals that are data (translations,
  matching rules, test inputs) may be in any language.
- The plugin only reads from Bangumi. Keep requests polite: one at a time, the minimum interval, the caches, and the
  back-off on `429` stay.

## Security issues

Do not open public issues for vulnerabilities; see [SECURITY.md](SECURITY.md).
