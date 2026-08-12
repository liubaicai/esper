package main

import (
	"os"

	manifestapp "github.com/liubaicai/esper/internal/app/manifest"
)

func main() {
	os.Exit(manifestapp.Run(os.Args[1:], os.Stdout, os.Stderr))
}
