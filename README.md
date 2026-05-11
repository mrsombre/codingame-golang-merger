# Source Code Merger

Source Code Merger is a command line tool that merges multiple source files from a project directory into a single file. Useful for coding competitions like [codingame](https://www.codingame.com) or any situation where you must submit a single source file.

Per-language adapters control how files are bundled. The Go adapter:

- picks up every `*.go` file in the source directory (`*_test.go` ignored),
- promotes `main.go` to the top of the output,
- emits a single `package` clause and a single deduped `import (...)` block,
- strips all comments (line, inline, block, doc).

## Usage

```shell
cgmerge [-s <source_dir>] [-o <output_file>] [-l <lang>]
```

If `--lang` is omitted, the language is auto-detected from a marker file in the source directory (e.g. `main.go` for Go).

If `--output` ends with the placeholder `.ext`, it is replaced by the adapter's real extension (so the default `bundle.ext` becomes `bundle.go`).

### Options

```shell
  -s, --source string   Source directory to parse (default ".")
  -o, --output string   Output file name (default "bundle.ext")
  -l, --lang string     Source language (auto-detect if empty)
  -h, --help            Show usage summary
  -v, --version         Show version
```

### Examples

Auto-detect language, write `./bundle.go` next to the sources:

```shell
cgmerge -s ./example/golang
```

Force the Go adapter, write to a custom path:

```shell
cgmerge -s ./cmd/bit -l go -o ./tmp/bit-bundle.go
# merged ./cmd/bit -> ./tmp/bit-bundle.go (10234 bytes)
```

Use the `.ext` placeholder so the adapter picks the extension:

```shell
cgmerge -s ./example/golang -o ./tmp/bundle.ext
# merged ./example/golang -> ./tmp/bundle.go (1421 bytes)
```

## License

[MIT](LICENSE) © 2023 Dmitrii Barsukov
