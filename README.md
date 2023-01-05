# Go File Merger

Go File Merger is a command line tool that allows you to merge multiple Go source files into a single file. This can be useful in coding competitions like [codingame](https://www.codingame.com) or other situations where you are limited to a single file for your code.

## Installation

### Linux / Mac

```shell
git clone https://github.com/mrsombre/codingame-golang-merger.git
cd codingame-golang-merger
go build -o bin/cgmerge
sudo mv bin/cgmerge /usr/local/bin/cgmerge
```

## Usage

To use Go File Merger, navigate to the directory containing the Go source files that you want to merge. Then, use the following command:

```shell
merger -source <output_filename> -dir <directory_name>
```

### Options

The following options are available:

```
- source string
    Name of the merged source file (default "_compiled.go")
- dir string
    Directory to parse (default ".")
```

## Tips

- Make sure that your code compiles and runs correctly before merging the files.
- The tool does not check for conflicts between files, so you need to make sure that there are no conflicts manually.
- The tool does not delete any of the original files, so you can keep them for reference.

## Contribution

If you find a bug or have an idea for a new feature, don't hesitate to open an issue or a pull request.
