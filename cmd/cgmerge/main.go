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
	var err error

	var optSourceName string
	var optDirName string
	var showHelp bool
	var showVersion bool

	flag.StringVarP(&optSourceName, "output", "o", "_merged.go", "Output file name")
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

	merger := internal.NewMerger()
	if err = merger.ParseDir(optDirName, optSourceName); err != nil {
		fmt.Println(err)
		return
	}

	if err = merger.WriteToFile(optSourceName); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Files merged successfully!")
}
