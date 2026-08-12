// Package sourcemanifest implements the source-manifest command.
package sourcemanifest

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/liubaicai/esper/internal/compat"
)

const defaultJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

// Run executes the source-manifest command and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("source-manifest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("source", `D:\Code\soc\esper`, "Esper Java source root")
	commit := flags.String("commit", defaultJavaCommit, "pinned Java commit")
	out := flags.String("out", "", "output JSON path; stdout when empty")
	if err := flags.Parse(args); err == flag.ErrHelp {
		return 0
	} else if err != nil {
		return 2
	}
	manifest, err := compat.ScanJavaSourceTests(*root, *commit)
	if err != nil {
		return fail(stderr, err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fail(stderr, err)
	}
	data = append(data, '\n')
	if *out == "" {
		if _, err := stdout.Write(data); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		return fail(stderr, err)
	}
	return 0
}

func fail(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintln(stderr, err)
	return 1
}
