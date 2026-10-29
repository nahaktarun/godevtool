<!-- What does this change do, and why? Link to the issue it addresses below. -->

## Summary

## Related issue

Fixes #

## Checklist

- [ ] Tests added or updated for this change
- [ ] `gofmt -l .` reports no files
- [ ] `go vet ./...` is clean
- [ ] Documentation updated (README, package doc comments, or examples)
- [ ] No new dependencies in the root module

<!--
CI runs gofmt, go vet, staticcheck and `go test -race ./...` on Go 1.21 and the
latest stable release. Running these locally first saves a round trip:

    gofmt -l .
    go vet ./...
    go test -race ./...

godevtool has no third-party dependencies and intends to keep it that way. If
your change needs one, open a feature request first so we can talk it through.
-->
