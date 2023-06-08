package main

import (
	"fmt"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/mrsombre/codingame-golang-merger/internal"
)

func main() {
	var err error

	var optSourceName string
	var optDirName string
	var isHelp bool

	flag.StringVarP(&optSourceName, "output", "o", "_merged.go", "Output file name")
	flag.StringVarP(&optDirName, "dir", "d", ".", "Source directory to parse")
	flag.BoolVarP(&isHelp, "help", "h", false, "Show usage summary")
	flag.Parse()

	if isHelp {
		fmt.Println("Usage: merger [options]")
		fmt.Println("Options:")
		flag.PrintDefaults()
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
