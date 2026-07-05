package main

import (
	"os"

	"dify-log-excel/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.ExecutableDir(), os.Stdout, os.Stderr))
}
