package main

// keyboard.go — SEARCH (button 1 below the screen) while the library is
// open = the top 4 pad rows type into mixxx's search box; again = hidden. mixxx has no control for search text, so keys go in as X key presses
// (XTEST) on :0, where mixxx has the focus. the mapping lights the pads,
// moves library focus to the search box on each key press and drops the
// deck functions of those rows while the library is open. this side follows
// the library + keyboard state from the same button presses the mapping
// sees (Session toggles the library, Add / Swap close it, SEARCH toggles
// the keyboard, closing the library hides it) and draws the key map over the bottom of the
// screen so you see which pad is which letter.

import (
	"image"
	"image/color"
	"log"
	"sync"
	"time"

	"github.com/federico-pepe/ableton-push-hack/core/gfx"
	"github.com/federico-pepe/ableton-push-hack/core/gfx/text"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

const (
	ccSession = 51 // left of the pads: library open / closed
	ccAdd     = 32 // load left deck, closes the library
	ccSwap    = 33 // load right deck, closes the library
	ccShift   = 49 // held = digits / symbols on the top two rows
	ccSearch  = 20 // button 1 below the screen: keyboard on / off (library open)
	kbFirst   = 68 // pad row 4 (from 0 at the bottom), first key
	kbRowH    = 14 // overlay row height, px

	symBackSpace = 0xff08
	symShiftL    = 0xffe1
	symControlL  = 0xffe3
	symClear     = 0 // not a keysym: ctrl+a, backspace
)

type key struct {
	label string
	sym   uint32
}

func k(s string) key { return key{s, uint32(s[0])} }

// kbRows: top pad row first, 8 keys each. ascii keysyms = the character.
// keep in sync with the mapping's keyboard colors (Ableton-Push-3.js kbColor).
var kbRows = [4][8]key{
	{k("a"), k("b"), k("c"), k("d"), k("e"), k("f"), k("g"), k("h")},
	{k("i"), k("j"), k("k"), k("l"), k("m"), k("n"), k("o"), k("p")},
	{k("q"), k("r"), k("s"), k("t"), k("u"), k("v"), k("w"), k("x")},
	{k("y"), k("z"), {"space", ' '}, {"space", ' '}, k("-"), k("'"), {"del", symBackSpace}, {"clear", symClear}},
}

// kbShift: the top two rows while Shift is held.
var kbShift = [2][8]key{
	{k("1"), k("2"), k("3"), k("4"), k("5"), k("6"), k("7"), k("8")},
	{k("9"), k("0"), k("."), k("&"), k("("), k(")"), k("/"), k("!")},
}

type keyboard struct {
	mu    sync.Mutex
	lib   bool // mirrors [Skin],show_maximized_library
	on    bool // keyboard shown (mirrors [PushHack],kb)
	shift bool
	down  int // note held, -1 none
	ver   int
	keys  chan key
}

func newKeyboard() *keyboard {
	kb := &keyboard{down: -1, keys: make(chan key, 64)}
	go kb.typeLoop()
	return kb
}

func (kb *keyboard) Stop() { close(kb.keys) }

// keyAt: key under a pad note, ok = note is in the keyboard rows.
func (kb *keyboard) keyAt(note int) (key, bool) {
	if note < kbFirst || note >= kbFirst+32 {
		return key{}, false
	}
	row, col := 3-(note-kbFirst)/8, (note-kbFirst)%8 // row 0 = top
	if kb.shift && row < 2 {
		return kbShift[row][col], true
	}
	return kbRows[row][col], true
}

func (kb *keyboard) CC(cc uint32, val int) {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	switch {
	case cc == ccSession && val > 0:
		kb.lib = !kb.lib
		kb.on = false
	case (cc == ccAdd || cc == ccSwap) && val > 0:
		kb.lib, kb.on = false, false
	case cc == ccSearch && val > 0 && kb.lib:
		kb.on = !kb.on
	case cc == ccShift:
		kb.shift = val > 0
	default:
		return
	}
	kb.ver++
}

func (kb *keyboard) Note(note int, on bool) {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	if !kb.on {
		return
	}
	key, ok := kb.keyAt(note)
	if !ok {
		return
	}
	if on {
		kb.down = note
		select {
		case kb.keys <- key:
		default: // typing fell behind: drop rather than block midi
		}
	} else if kb.down == note {
		kb.down = -1
	}
	kb.ver++
}

// LibOpen: the library is open (as far as the button presses tell).
func (kb *keyboard) LibOpen() bool {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	return kb.lib
}

// State: keyboard shown?, change counter (overlay redraw).
func (kb *keyboard) State() (bool, int) {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	return kb.on, kb.ver
}

// Draw: key map over the bottom of the frame, one cell per pad, pad columns
// = 120 px screen columns. held key orange.
func (kb *keyboard) Draw(img *image.NRGBA) {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	y0 := screenH - 4*kbRowH
	gfx.FillRect(img, 0, y0-1, screenW, 4*kbRowH+1, color.NRGBA{0, 0, 0, 255})
	for n := kbFirst; n < kbFirst+32; n++ {
		key, _ := kb.keyAt(n)
		row, col := 3-(n-kbFirst)/8, (n-kbFirst)%8
		x, y := col*120, y0+row*kbRowH
		bg, fg := color.NRGBA{30, 30, 30, 255}, color.NRGBA{220, 220, 220, 255}
		switch key.label {
		case "space":
			bg = color.NRGBA{20, 40, 80, 255}
		case "del", "clear":
			bg = color.NRGBA{80, 20, 20, 255}
		}
		if n == kb.down {
			bg, fg = color.NRGBA{255, 85, 0, 255}, color.NRGBA{0, 0, 0, 255}
		}
		gfx.FillRect(img, x+1, y+1, 118, kbRowH-2, bg)
		text.Draw(img, x+60-text.Width(key.label)/2, y+kbRowH-3, key.label, fg)
	}
}

// typeLoop sends keys as X key presses, in order. opens :0 on first use.
func (kb *keyboard) typeLoop() {
	var x *xgb.Conn
	var root xproto.Window
	codes := map[uint32][2]byte{} // keysym -> keycode, shift level
	defer func() {
		if x != nil {
			x.Close()
		}
	}()
	tap := func(code byte, mods ...byte) {
		for _, m := range mods {
			xtest.FakeInput(x, xproto.KeyPress, m, 0, root, 0, 0, 0)
		}
		xtest.FakeInput(x, xproto.KeyPress, code, 0, root, 0, 0, 0)
		xtest.FakeInput(x, xproto.KeyRelease, code, 0, root, 0, 0, 0)
		for _, m := range mods {
			xtest.FakeInput(x, xproto.KeyRelease, m, 0, root, 0, 0, 0)
		}
		x.Sync()
	}
	for key := range kb.keys {
		if x == nil {
			c, err := xgb.NewConnDisplay(":0")
			if err == nil {
				err = xtest.Init(c)
			}
			if err != nil {
				log.Printf("keyboard: %v", err)
				if c != nil {
					c.Close()
				}
				continue
			}
			x = c
			s := xproto.Setup(x)
			root = s.DefaultScreen(x).Root
			n := int(s.MaxKeycode - s.MinKeycode + 1)
			m, err := xproto.GetKeyboardMapping(x, s.MinKeycode, byte(n)).Reply()
			if err != nil {
				log.Printf("keyboard: %v", err)
				continue
			}
			per := int(m.KeysymsPerKeycode)
			for i := 0; i < n; i++ {
				for lvl := 0; lvl < 2 && lvl < per; lvl++ {
					sym := uint32(m.Keysyms[i*per+lvl])
					if _, seen := codes[sym]; !seen && sym != 0 {
						codes[sym] = [2]byte{byte(s.MinKeycode) + byte(i), byte(lvl)}
					}
				}
			}
		}
		// the mapping moves focus to the search box on the same pad press;
		// give mixxx a moment so the first letter lands there.
		time.Sleep(30 * time.Millisecond)
		if key.sym == symClear {
			tap(codes['a'][0], codes[symControlL][0])
			tap(codes[symBackSpace][0])
			continue
		}
		c, ok := codes[key.sym]
		if !ok {
			continue
		}
		if c[1] == 1 {
			tap(c[0], codes[symShiftL][0])
		} else {
			tap(c[0])
		}
	}
}
