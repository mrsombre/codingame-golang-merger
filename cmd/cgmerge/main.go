package main

import (
	"fmt"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/mrsombre/codingame-golang-merger/internal"
)

var (
	version = "unknown"
	date    = "unknown"
)

func main() {
	var optOutput string
	var optSource string
	var optLang string
	var showHelp bool
	var showVersion bool

	flag.StringVarP(&optOutput, "output", "o", "bundle.ext", "Output file name")
	flag.StringVarP(&optSource, "source", "s", ".", "Source directory to parse")
	flag.StringVarP(&optLang, "lang", "l", "", "Source language (auto-detect if empty)")
	flag.BoolVarP(&showHelp, "help", "h", false, "Show usage summary")
	flag.BoolVarP(&showVersion, "version", "v", false, "Show version")
	flag.Parse()

	if showHelp {
		fmt.Println("Usage: cgmerge [options]")
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if showVersion {
		fmt.Printf("cgmerge version %s @ %s\n", version, date)
		os.Exit(0)
	}

	out, err := internal.Run(optSource, optOutput, optLang)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
	info, err := os.Stat(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("merged %s -> %s (%d bytes)\n", optSource, out, info.Size())
}
