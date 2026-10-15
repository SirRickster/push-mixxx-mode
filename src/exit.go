package main

// exit.go — hold SETTINGS for 1.5 s in mixxx mode -> back to push mode.
// also feeds HELP presses and jog turns to the manual (manual.go), and
// pads + library buttons to the search keyboard (keyboard.go), IMPORT to
// the playlist import (import.go).
// own alsa client subscribed to the push port; mixxx reads the same port.
// only opened in mixxx mode, long after boot, so no usb-a boot-settle wait.

import (
	"encoding/binary"
	"log"
	"sync"
	"time"

	"github.com/federico-pepe/ableton-push-hack/core/alsaseq"
)

const (
	ccSettings = 30
	ccHelp     = 81
	ccJog      = 70
	ccImport   = 21 // button 2 below the screen, library open (import.go)
	exitHold   = 1500 * time.Millisecond
	pushPort   = "Ableton Push 3 Live Port"
)

type exitWatch struct {
	c     *alsaseq.Client
	fire  func()
	man   *manual
	kb    *keyboard
	imp   *importer
	mu    sync.Mutex
	timer *time.Timer
}

func startExitWatch(fire func(), man *manual, kb *keyboard, imp *importer) *exitWatch {
	port, ok := alsaseq.FindByName(pushPort, 0)
	if !ok {
		log.Printf("exit watch: %q not found, use the web page to go back", pushPort)
		return nil
	}
	c, err := alsaseq.Open()
	if err != nil {
		log.Printf("exit watch: %v", err)
		return nil
	}
	if _, err := c.CreatePort("Mixxx Bridge In", alsaseq.CapWrite|alsaseq.CapSubsWrite, alsaseq.PortTypeMidi|alsaseq.PortTypeApp); err != nil {
		log.Printf("exit watch: %v", err)
		c.Close()
		return nil
	}
	if err := c.Subscribe(port.Addr); err != nil {
		log.Printf("exit watch: %v", err)
		c.Close()
		return nil
	}
	w := &exitWatch{c: c, fire: fire, man: man, kb: kb, imp: imp}
	go c.ReadLoop(w) //nolint:errcheck // ends when Stop closes fd
	return w
}

func (w *exitWatch) Stop() {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.timer != nil {
		w.timer.Stop()
	}
	w.mu.Unlock()
	w.c.Close()
}

func (w *exitWatch) Fixed(evType uint8, _ alsaseq.Addr, data []byte) {
	if evType == alsaseq.EvNoteOn || evType == alsaseq.EvNoteOff {
		// note event: channel, note, velocity
		w.kb.Note(int(data[1]), evType == alsaseq.EvNoteOn && data[2] > 0)
		return
	}
	if evType != alsaseq.EvController {
		return
	}
	cc, val := binary.LittleEndian.Uint32(data[4:]), int(int32(binary.LittleEndian.Uint32(data[8:])))
	switch {
	case cc == ccHelp && val > 0:
		w.man.Toggle()
		return
	case cc == ccJog:
		if val >= 64 {
			val -= 128 // relative: 1 cw, 127 ccw
		}
		w.man.Scroll(val)
		return
	case cc == ccImport && val > 0 && w.kb.LibOpen():
		w.imp.Start()
		return
	case cc != ccSettings:
		w.kb.CC(cc, val)
		return
	}
	pressed := val > 0
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	if pressed {
		// fire in own goroutine: ToPush stops this watcher.
		w.timer = time.AfterFunc(exitHold, func() { go w.fire() })
	}
}

func (w *exitWatch) VarLen(uint8, alsaseq.Addr, []byte) {}
