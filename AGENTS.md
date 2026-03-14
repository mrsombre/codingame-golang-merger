# CodinGame Go Source Bundler

## Project Overview

CLI tool that merges multiple Go source files into a single file for CodinGame submissions. Parses a Go project directory, inlines local packages, eliminates dead code, strips comments, and produces a single bundled `.go` file.

## Project Structure

```
cmd/cgmerge/main.go         # CLI entrypoint
internal/                   # source
example/
├─ flat/                    # Example: single-package project
└─ nested/                  # Example: multi-package project with cmd/alpha/beta
Taskfile.yml                # Task runner (build, test)
```

## Project Commands

```shell
task test                   # Run all tests
task build                  # Build binary to ./bin/cgmerge
```
