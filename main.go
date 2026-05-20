package main

import (
	"os"
	"runtime/debug"

	"github.com/korECM/openapi-extract/internal/cli"
)

// version is set at build time via -ldflags "-X main.version=vX.Y.Z" for
// release binaries. When unset (e.g. plain `go install`), Run falls back to
// the module's BuildInfo.Main.Version.
var version = ""

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, cli.RunOptions{
		Version: resolveVersion(),
	}))
}

func resolveVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "(devel)"
}
