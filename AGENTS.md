# CodinGame Source Bundler

## Project Overview

CLI tool that merges multiple source files from a project directory into a single submission file (e.g. for CodinGame). Language-agnostic core with per-language adapters. The Go adapter parses sources via `go/parser`, deduplicates imports into one block, promotes `main.go` to the top, and strips all comments.

## Project Structure

```
cmd/
└─ cgmerge/             # CLI entrypoint (flags, dispatch)
internal/               # Adapter interface, registry, merger orchestration
├─ adapter.go           # Adapter interface + Register/Get/Detect registry
├─ golang.go            # Go language adapter (init-registers itself)
└─ merger.go            # Run(): resolve adapter, find files, write bundle
example/
└─ golang/              # Sample multi-file Go project used for smoke tests
Makefile                # vet / test / lint / build targets
```

## Project Rules

- NEVER commit changes without explicit user instruction
- ALWAYS write smoke-test bundles to `./tmp/` (gitignored), not project root
- ALWAYS keep adapters self-registering via `init()` so `cmd/cgmerge` stays adapter-agnostic
- ALWAYS treat the first entry of `Adapter.Markers()` as both the auto-detect marker and the file promoted to position 0 in the merged output

## Project Commands

```shell
make build              # Build binary to ./bin/cgmerge
make test               # Run all tests
make vet                # go vet
make lint               # golangci-lint
```

## Smoke Test

```shell
make build && ./bin/cgmerge -s ./example/golang -o ./tmp/bundle.ext
go run ./tmp/bundle.go
```
