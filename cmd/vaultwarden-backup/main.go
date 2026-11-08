package main

import (
	"os"

	"github.com/mniak/vw-backup-go/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
