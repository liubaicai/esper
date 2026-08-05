package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/liubaicai/esper/compat"
)

func main() {
	root := flag.String("source", `D:\Code\soc\esper`, "Esper Java source root")
	commit := flag.String("commit", "9e1b9f1cc9117fea4bf33ab043762c045d73839c", "pinned Java commit")
	out := flag.String("out", "", "output JSON path; stdout when empty")
	flag.Parse()
	manifest, err := compat.ScanJavaSourceTests(*root, *commit)
	if err != nil {
		fail(err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fail(err)
	}
	data = append(data, '\n')
	if *out == "" {
		_, _ = os.Stdout.Write(data)
		return
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
