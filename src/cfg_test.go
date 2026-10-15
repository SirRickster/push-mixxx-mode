package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCfgSet(t *testing.T) {
	p := filepath.Join(t.TempDir(), "mixxx.cfg")
	os.WriteFile(p, []byte("[Config]\nResizableSkin LateNight\nhide_menubar 0\n\n[Master]\nnum_decks 2\n"), 0o644)
	for _, kv := range [][3]string{
		{"[Config]", "ResizableSkin", "PushHack"},         // replace
		{"[Config]", "show_menubar_hint", "0"},            // add to existing group
		{"[Controller]", "Ableton_Push_3_Live_Port", "1"}, // new group
		{"[Config]", "hide_menubar", "1"},
	} {
		if err := cfgSet(p, kv[0], kv[1], kv[2]); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := os.ReadFile(p)
	want := "[Config]\nshow_menubar_hint 0\nResizableSkin PushHack\nhide_menubar 1\n\n[Master]\nnum_decks 2\n\n[Controller]\nAbleton_Push_3_Live_Port 1\n"
	if string(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSeedEffects(t *testing.T) {
	st := t.TempDir()
	if err := seedEffects(st); err != nil { // no presets yet: skip quietly
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(st, "effects.xml")); err == nil {
		t.Fatal("wrote effects.xml without presets")
	}
	os.MkdirAll(filepath.Join(st, "effects", "chains"), 0o755)
	for _, n := range []string{"Reverb", "Echo", "Flanger"} {
		os.WriteFile(filepath.Join(st, "effects", "chains", n+".xml"),
			[]byte("<EffectChain><Effects><Effect><Id>"+n+"</Id></Effect></Effects></EffectChain>"), 0o644)
	}
	if err := seedEffects(st); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(st, "effects.xml"))
	unit := "<Effect><Id>Reverb</Id></Effect><Effect><Id>Echo</Id></Effect><Effect><Id>Flanger</Id></Effect>"
	if strings.Count(string(got), unit) != 4 || !strings.HasPrefix(string(got), "<?xml") ||
		strings.Count(string(got), "bessel8lvmixeq Built-In") != 4 {
		t.Fatalf("got:\n%s", got)
	}
}

func TestAudioSettings(t *testing.T) {
	st := t.TempDir()
	if a := loadAudio(st); a != audioDefault {
		t.Fatalf("no file: %+v", a)
	}
	// default: master 1/2 (channel 0), cue 3/4 = headphone jack (channel 2), 256 frames = index 3
	x := soundConfig(audioDefault)
	if !strings.Contains(x, `latency="3"`) || !strings.Contains(x, `channel="0" type="Master"`) || !strings.Contains(x, `channel="2" type="Headphones"`) {
		t.Fatalf("default:\n%s", x)
	}
	if err := saveAudio(st, audioCfg{Buffer: 384, Master: "1/2"}); err == nil {
		t.Fatal("384 frames accepted")
	}
	if err := saveAudio(st, audioCfg{Buffer: 64, Master: "3/4"}); err != nil {
		t.Fatal(err)
	}
	a := loadAudio(st)
	if x := soundConfig(a); !strings.Contains(x, `latency="1"`) || !strings.Contains(x, `channel="2" type="Master"`) {
		t.Fatalf("saved %+v:\n%s", a, x)
	}
}
