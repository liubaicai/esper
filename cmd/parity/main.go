package main

import (
	"os"

	parityapp "github.com/liubaicai/esper/internal/app/parity"
)

func main() {
	os.Exit(parityapp.Run(os.Args[1:], os.Stdout, os.Stderr))
}
