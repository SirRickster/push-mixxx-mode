package main

// cfg.go — set one key in mixxx.cfg ("[Group]" lines, then "key value"),
// and the audio settings behind soundconfig.xml.

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math/bits"
	"os"
	"path/filepath"
	"strings"
)

func cfgSet(path, group, key, value string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(data) == 0 {
		lines = nil
	}
	in, gi := false, -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") {
			in = t == group
			if in {
				gi = i
			}
			continue
		}
		if in && (t == key || strings.HasPrefix(t, key+" ")) {
			lines[i] = key + " " + value
			return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
		}
	}
	if gi < 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, group, key+" "+value)
	} else {
		lines = append(lines[:gi+1], append([]string{key + " " + value}, lines[gi+1:]...)...)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// copyDir copies src tree into dst, overwriting files.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, data, 0o644)
	})
}

// audioCfg: the web page's audio settings, settings/audio.json (settings/
// survives catalog updates). soundconfig.xml is written from it on every
// mixxx start, so a change applies at the next start.
type audioCfg struct {
	Buffer int    `json:"buffer"` // frames, 64..4096, power of two
	Master string `json:"master"` // "1/2" or "3/4"; cue (headphones) gets the other pair
}

// 256 frames = 5.8 ms at 44.1 kHz. crackles -> 512. mixxx's own default
// (1024, ~50 ms press-to-sound) was all it could do while its audio thread
// had no realtime priority.
var audioDefault = audioCfg{Buffer: 256, Master: "1/2"} // cue 3/4 = headphone jack

func (a audioCfg) check() error {
	if a.Buffer < 64 || a.Buffer > 4096 || a.Buffer&(a.Buffer-1) != 0 {
		return fmt.Errorf("buffer must be 64 to 4096 frames, a power of two")
	}
	if a.Master != "1/2" && a.Master != "3/4" {
		return fmt.Errorf(`master must be "1/2" or "3/4"`)
	}
	return nil
}

func loadAudio(st string) audioCfg {
	a := audioDefault
	if data, err := os.ReadFile(filepath.Join(st, "audio.json")); err == nil {
		if json.Unmarshal(data, &a) != nil || a.check() != nil {
			return audioDefault
		}
	}
	return a
}

func saveAudio(st string, a audioCfg) error {
	if err := a.check(); err != nil {
		return err
	}
	if err := os.MkdirAll(st, 0o755); err != nil {
		return err
	}
	data, _ := json.Marshal(a)
	return os.WriteFile(filepath.Join(st, "audio.json"), data, 0o644)
}

func writeSoundConfig(path string, a audioCfg) error {
	if err := os.WriteFile(path, []byte(soundConfig(a)), 0o644); err != nil {
		return err
	}
	return chownTree(path, mixxxUser) // mixxx saves it from its own preferences
}

// soundConfig: mixxx's sound setup for the push's own card. latency is
// mixxx's buffer index: frames = 64 << (index - 1).
func soundConfig(a audioCfg) string {
	master, cue := 2, 0
	if a.Master == "1/2" {
		master, cue = 0, 2
	}
	return fmt.Sprintf(`<!DOCTYPE SoundManagerConfig>
<SoundManagerConfig deck_count="4" api="ALSA" latency="%d" sync_buffers="2" force_network_clock="0" samplerate="44100">
 <SoundDevice portAudioIndex="0" alsaHwDevice="hw:0,0" name="Ableton Push 3: USB Audio">
  <output channel_count="2" channel="%d" type="Master" index="0"/>
  <output channel_count="2" channel="%d" type="Headphones" index="0"/>
 </SoundDevice>
</SoundManagerConfig>
`, bits.TrailingZeros(uint(a.Buffer))-5, master, cue)
}
