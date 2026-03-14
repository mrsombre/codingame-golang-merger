package main

import (
	"fmt"
	"os"
	"path/filepath"

	flag "github.com/spf13/pflag"

	"github.com/mrsombre/codingame-golang-merger/internal"
)

var (
	version = "unknown"
	date    = "unknown"
)

func main() {
	var err error

	var optSourceName string
	var optDirName string
	var showHelp bool
	var showVersion bool

	flag.StringVarP(&optSourceName, "output", "o", "bundle.go", "Output file name")
	flag.StringVarP(&optDirName, "dir", "d", ".", "Source directory to parse")
	flag.BoolVarP(&showHelp, "help", "h", false, "Show usage summary")
	flag.BoolVarP(&showVersion, "version", "v", false, "Show version")
	flag.Parse()

	if showHelp {
		fmt.Println("Usage: merger [options]")
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if showVersion {
		fmt.Printf("cgmerge version %s @ %s\n", version, date)
		os.Exit(0)
	}

	if !flag.CommandLine.Changed("output") {
		optSourceName = filepath.Join(optDirName, optSourceName)
	}

	merger := internal.NewMerger()
	if err = merger.ParseDir(optDirName, optSourceName); err != nil {
		fmt.Println(err)
		return
	}

	if err = merger.WriteToFile(optSourceName); err != nil {
		fmt.Println(err)
		return
	}

	absDirMain, _ := filepath.Abs(filepath.Join(optDirName, "main.go"))
	absOutput, _ := filepath.Abs(optSourceName)
	info, _ := os.Stat(optSourceName)
	fmt.Printf("merged %s ->\n  %s (%d chars)\n", absDirMain, absOutput, info.Size())
}
