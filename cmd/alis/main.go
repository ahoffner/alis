// Command alis turns packs of aliases, functions and scripts into shell shortcuts.
package main

import (
	"os"

	"github.com/ahoffner/alis/internal/cli"
)

// Set at build time: go build -ldflags "-X main.version=… -X main.shareDir=…".
var (
	version  = "dev"
	shareDir = ""
)

func main() {
	os.Exit(cli.Execute(cli.Build{Version: version, ShareDir: shareDir}))
}
