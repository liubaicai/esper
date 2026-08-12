package main

import (
	"os"

	sourcemanifestapp "github.com/liubaicai/esper/internal/app/sourcemanifest"
)

func main() {
	os.Exit(sourcemanifestapp.Run(os.Args[1:], os.Stdout, os.Stderr))
}
