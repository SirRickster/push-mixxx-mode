package main

import (
	"os"
	"strings"
	"testing"
)

// the real README: Controls section only, tables flattened, fits the screen.
func TestManualLines(t *testing.T) {
	data, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatal(err)
	}
	lines := manualLines(string(data))
	all := strings.Join(lines, "\n")
	if len(lines) < manualRows || strings.Contains(all, "|") || strings.Contains(all, "**") ||
		strings.Contains(all, "## Music") || !strings.Contains(all, "Play / cue: Play / Record") {
		t.Fatalf("got:\n%s", all)
	}
	for _, l := range lines {
		if len(l) > manualCols {
			t.Fatalf("too wide (%d): %q", len(l), l)
		}
	}
}
