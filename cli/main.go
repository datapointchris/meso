package main

import (
	"os"

	"meso/cli/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
