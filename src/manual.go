package main

// manual.go — HELP in mixxx mode: README's Controls section on the push
// screen instead of the mixxx window, jog wheel scrolls, HELP again closes.
// drawn here, not in the skin: a skin has no scrollable text widget. the
// mapping ignores the jog while it is open (both see the same HELP presses).

import (
	"image"
	"image/color"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/federico-pepe/ableton-push-hack/core/gfx"
	"github.com/federico-pepe/ableton-push-hack/core/gfx/text"
)

const (
	manualRows = 11  // text rows under the header, 13 px each
	manualCols = 134 // 7 px per char, 8 px margins
)

type manual struct {
	path string // README.md in the hack dir, read on every open

	mu    sync.Mutex
	on    bool
	top   int
	ver   int // bumped on every change; grabber redraws when it moves
	lines []string
}

func (m *manual) Toggle() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.on {
		data, err := os.ReadFile(m.path)
		if err != nil {
			m.lines = []string{"No manual: " + err.Error()}
		} else {
			m.lines = manualLines(string(data))
		}
		m.top = 0
	}
	m.on = !m.on
	m.ver++
}

func (m *manual) Scroll(d int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.on {
		return
	}
	m.top = max(0, min(m.top+d, len(m.lines)-manualRows))
	m.ver++
}

// State: open?, change counter.
func (m *manual) State() (bool, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.on, m.ver
}

func (m *manual) Frame() *image.NRGBA {
	m.mu.Lock()
	defer m.mu.Unlock()
	img := image.NewNRGBA(image.Rect(0, 0, screenW, screenH))
	gfx.FillRect(img, 0, 0, screenW, screenH, color.NRGBA{0, 0, 0, 255})
	end := min(m.top+manualRows, len(m.lines))
	head := "Manual - jog scrolls, Help closes"
	pos := strconv.Itoa(m.top+1) + "-" + strconv.Itoa(end) + " / " + strconv.Itoa(len(m.lines))
	text.Draw(img, 8, 12, head, color.NRGBA{255, 85, 0, 255})
	text.Draw(img, screenW-8-text.Width(pos), 12, pos, color.NRGBA{120, 120, 120, 255})
	for i, l := range m.lines[m.top:end] {
		text.Draw(img, 8, 28+i*13, l, color.NRGBA{220, 220, 220, 255})
	}
	return img
}

// manualLines: the "## Controls" section of README as screen lines. tables
// become "first cell: rest / rest", markdown marks go, long lines wrap.
func manualLines(md string) []string {
	if i := strings.Index(md, "\n## Controls"); i >= 0 {
		md = md[i+1:]
		md = md[strings.Index(md, "\n")+1:] // drop the heading itself
		if j := strings.Index(md, "\n## "); j >= 0 {
			md = md[:j]
		}
	}
	md = strings.NewReplacer("**", "", "`", "").Replace(md)
	var out []string
	blank := true
	for _, l := range strings.Split(md, "\n") {
		l = strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, "|-") || strings.HasPrefix(l, "|:"):
			continue
		case strings.HasPrefix(l, "|"):
			var cells []string
			for _, c := range strings.Split(strings.Trim(l, "|"), "|") {
				cells = append(cells, strings.TrimSpace(c))
			}
			l = cells[0] + ": " + strings.Join(cells[1:], " / ")
		case strings.HasPrefix(l, "#"):
			l = strings.TrimLeft(l, "# ")
		}
		if l == "" {
			if !blank {
				out = append(out, "")
			}
			blank = true
			continue
		}
		blank = false
		out = append(out, wrap(l, manualCols)...)
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

func wrap(s string, n int) []string {
	var out []string
	line := ""
	for _, w := range strings.Fields(s) {
		if line != "" && len(line)+1+len(w) > n {
			out = append(out, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += w
	}
	return append(out, line)
}
