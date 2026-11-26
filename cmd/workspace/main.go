package main

import (
	"errors"
	"os"
	"runtime/debug"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/release"
)

var version = "dev"

func main() {
	// A release's build is stamped with its tag (release.yml), and is the
	// kind ws upgrade replaces. One built by go install says its module's
	// version, which looks the same and isn't.
	cli.ReleaseBuild = release.IsTag(version)
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
			version = info.Main.Version
		}
	}
	cmd := cli.NewRootCmd(version)
	if err := cmd.Execute(); err != nil {
		if exit, ok := errors.AsType[*cli.ExitCodeError](err); ok {
			os.Exit(exit.Code)
		}
		os.Exit(1)
	}
}
