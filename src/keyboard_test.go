package main

import "testing"

func TestKeyboardLayout(t *testing.T) {
	kb := &keyboard{down: -1, keys: make(chan key, 8)}
	// top-left pad (note 92) = a, shift = 1; row 4 last pad (note 75) = clear
	if k, _ := kb.keyAt(92); k.label != "a" {
		t.Fatalf("92 = %q", k.label)
	}
	if k, _ := kb.keyAt(75); k.sym != symClear {
		t.Fatalf("75 = %q", k.label)
	}
	if _, ok := kb.keyAt(67); ok {
		t.Fatal("row 3 is not a key")
	}
	// keyboard hidden: pads type nothing, even with the library open;
	// SEARCH shows it (only while the library is open); Add closes both
	kb.CC(ccSearch, 127)
	kb.CC(ccSession, 127)
	kb.Note(92, true)
	if len(kb.keys) != 0 {
		t.Fatal("typed with keyboard hidden")
	}
	kb.CC(ccSearch, 127)
	kb.CC(ccShift, 127)
	kb.Note(92, true)
	if k := <-kb.keys; k.label != "1" {
		t.Fatalf("shift+92 = %q", k.label)
	}
	kb.CC(ccAdd, 127)
	if on, _ := kb.State(); on || kb.lib {
		t.Fatal("Add should close the library and the keyboard")
	}
}
