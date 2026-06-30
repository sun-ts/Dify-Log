package main

import (
	"fmt"
	"os"

	"dify-log-excel/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version.Version)
		return
	}

	fmt.Fprintln(os.Stderr, "usage: dify-log-excel <serve|sync|status|version>")
	os.Exit(2)
}
