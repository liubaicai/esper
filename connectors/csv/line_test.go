package csv

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/liubaicai/esper/connectors"
)

func TestLineSourcePreservesBlankLinesAndLoops(t *testing.T) {
	source, err := NewLineSource(LineSourceSpec{Reader: bytes.NewReader([]byte("first\n\nlast")), Loop: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	want := []string{"first", "", "last", "first"}
	for index, expected := range want {
		got, err := source.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if got != expected {
			t.Fatalf("line %d = %q, want %q", index, got, expected)
		}
	}
	if err := source.Reset(); err != nil {
		t.Fatal(err)
	}
	if got, err := source.Next(context.Background()); err != nil || got != "first" {
		t.Fatalf("reset line=%q err=%v", got, err)
	}
}

func TestLineSourceRunAndEOFState(t *testing.T) {
	source, err := NewLineSource(LineSourceSpec{Reader: strings.NewReader("a\nb\n")})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	var lines []string
	if err := source.Run(context.Background(), func(_ context.Context, line string) error {
		lines = append(lines, line)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || lines[0] != "a" || lines[1] != "b" {
		t.Fatalf("lines = %#v", lines)
	}
	if source.State() != connectors.Opened {
		t.Fatalf("state = %s, want OPENED", source.State())
	}
	if _, err := source.Next(context.Background()); !errors.Is(err, connectors.ErrNotStarted) {
		t.Fatalf("Next after EOF = %v", err)
	}
}
