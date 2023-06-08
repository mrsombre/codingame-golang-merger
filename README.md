# Go File Merger

Go File Merger is a command line tool that allows you to merge multiple Go source files into a single file. This can be useful in coding competitions like [codingame](https://www.codingame.com) or other situations where you are limited to a single file for your code.

## Installation

### Linux / Mac / WSL

```shell
git clone https://github.com/mrsombre/codingame-golang-merger.git
cd codingame-golang-merger
go build -o bin/cgmerge ./cmd/cgmerge
sudo mv bin/cgmerge /usr/local/bin/cgmerge
```

### Example

```shell
cd example/simple
cgmerge
```

## Usage

To use Go File Merger, navigate to the directory containing the Go source files that you want to merge. Then, use the following command:

```shell
cgmerge [--output <output_filename>] [--dir <source_directory_name>]
```

### Options

```shell
  -d, --dir string      Source directory to parse (default ".")
  -o, --output string   Output file name (default "_merged.go")
  -h, --help            Show usage summary
```

## Notes

- This tool could merge only files from one directory using `main` package namespace and did not merge files imported from other packages.
- This tool does not check for syntax errors, so make sure that your code is syntactically correct before merging the files.
- The tool does not check for conflicts between files, so you need to make sure that there are no conflicts manually.
- The tool does not delete any of the original files, so you can keep them for reference.

## Contribution

If you find a bug or have an idea for a new feature, don't hesitate to open an issue or a pull request.
