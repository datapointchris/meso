package main

import (
	"os"

	"github.com/datapointchris/meso/cli/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
