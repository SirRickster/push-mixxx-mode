package main

// mode.go — the two switch sequences. order matters:
//   to mixxx: stop push3 -> push-manager claims screen -> start mixxx
//   to push:  stop mixxx -> push-manager releases screen -> start push3
// push3 claims the screen's usb interface at start, so screen must be
// released before push3 starts or push3 comes up without a screen.

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/federico-pepe/ableton-push-hack/core/gfx"
	"github.com/federico-pepe/ableton-push-hack/core/gfx/text"
)

const (
	modePush  = "push"
	modeMixxx = "mixxx"

	push3Init = "/etc/init.d/push3"
	pmDisplay = "http://127.0.0.1:7701/api/display/"
	mixxxUser = "ableton"
	musicDir  = "/data/Music/DJ"

	// RLIMIT_RTPRIO (not in package syscall). mixxx is exec'd straight as
	// ableton, no pam login, so limits.conf never gave it realtime: portaudio
	// asked for SCHED_FIFO and was refused. set on the bridge, children inherit.
	rlimitRTPrio = 14
	maxRTPrio    = 95
)

type bridge struct {
	dir string // hack dir: mixxx bundle, mapping/, settings/

	mu    sync.Mutex
	mode  string
	mixxx *exec.Cmd
	exit  *exitWatch
	grab  *grabber
	man   *manual
	kb    *keyboard
	usb   *usbWatch
	imp   *importer
}

func newBridge(dir string) *bridge { return &bridge{dir: dir, mode: modePush} }

func (b *bridge) Mode() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.mode
}

func (b *bridge) ToMixxx() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.mode == modeMixxx {
		return nil
	}
	log.Printf("switching to mixxx mode")
	if err := run(push3Init, "stop"); err != nil {
		return fmt.Errorf("stop push3: %w", err)
	}
	if !waitNoProc("Push3", 5*time.Second) {
		run(push3Init, "start") //nolint:errcheck
		return fmt.Errorf("Push3 did not stop, left in push mode")
	}
	if err := pmDirect(true); err != nil {
		log.Printf("screen: %v (continuing without screen)", err)
	} else {
		showMessage("MIXXX MODE", "Starting Mixxx...", "Hold SETTINGS to go back to Push")
	}
	if err := b.seedSettings(); err != nil {
		log.Printf("settings: %v", err)
	}
	if err := b.ensureLibrary(); err != nil {
		log.Printf("library: %v", err)
	}
	b.usb = startUSBWatch()
	st := filepath.Join(b.dir, "settings")
	relocate(st) // copies from the last import: mixxx is stopped now
	cmd, err := b.startMixxx()
	if err != nil {
		b.usb.Stop()
		b.usb = nil
		b.restorePush()
		return fmt.Errorf("start mixxx: %w", err)
	}
	b.mixxx = cmd
	b.mode = modeMixxx
	b.man = &manual{path: filepath.Join(b.dir, "README.md")}
	b.kb = newKeyboard()
	b.imp = newImporter(st)
	b.grab = startGrab(cmd.Process.Pid, b.man, b.kb, b.imp)
	b.exit = startExitWatch(func() {
		if err := b.ToPush(); err != nil {
			log.Printf("exit: %v", err)
		}
	}, b.man, b.kb, b.imp)
	go func(c *exec.Cmd) { // mixxx dies on its own -> say so on screen
		c.Wait() //nolint:errcheck
		b.mu.Lock()
		still := b.mixxx == c
		b.mu.Unlock()
		if still {
			log.Printf("mixxx exited")
			b.mu.Lock()
			if b.grab != nil {
				b.grab.Stop()
				b.grab = nil
			}
			b.mu.Unlock()
			showMessage("MIXXX STOPPED", "Mixxx quit or crashed.", "Hold SETTINGS to go back to Push")
		}
	}(cmd)
	return nil
}

func (b *bridge) ToPush() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.mode == modePush {
		return nil
	}
	log.Printf("switching to push mode")
	if b.exit != nil {
		b.exit.Stop()
		b.exit = nil
	}
	if b.grab != nil {
		b.grab.Stop()
		b.grab = nil
	}
	if b.kb != nil {
		b.kb.Stop() // after exit + grab: nothing sends keys any more
		b.kb = nil
	}
	if b.imp != nil {
		b.imp.Stop() // before usb: a copy holds files on the stick
		b.imp = nil
	}
	if b.mixxx != nil {
		showMessage("PUSH MODE", "Stopping Mixxx, starting Push...", "")
		stopGroup(b.mixxx, 10*time.Second)
		b.mixxx = nil
		relocate(filepath.Join(b.dir, "settings"))
	}
	if b.usb != nil {
		b.usb.Stop() // after mixxx: it no longer holds files on the sticks
		b.usb = nil
	}
	b.restorePush()
	b.mode = modePush
	return nil
}

// restorePush releases the screen, then starts push3. caller holds mu.
func (b *bridge) restorePush() {
	if err := pmDirect(false); err != nil {
		log.Printf("screen release: %v", err)
	}
	if err := run(push3Init, "start"); err != nil {
		log.Printf("start push3: %v", err)
	}
}

// seedSettings puts the push 3 mapping where mixxx finds it, and on first
// run writes a mixxx.cfg that enables it. after that mixxx owns mixxx.cfg.
func (b *bridge) seedSettings() error {
	st := filepath.Join(b.dir, "settings")
	ctl := filepath.Join(st, "controllers")
	if err := os.MkdirAll(ctl, 0o755); err != nil {
		return err
	}
	for _, f := range []string{"Ableton-Push-3.midi.xml", "Ableton-Push-3.js"} {
		data, err := os.ReadFile(filepath.Join(b.dir, "mapping", f))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(ctl, f), data, 0o644); err != nil {
			return err
		}
	}
	// library = musicDir only. all of /data/Music would pull in ableton's
	// sample library as thousands of "tracks".
	if err := os.MkdirAll(musicDir, 0o755); err != nil {
		return err
	}
	if err := copyDir(filepath.Join(b.dir, "skin"), filepath.Join(st, "skins")); err != nil {
		return err
	}
	cfg := filepath.Join(st, "mixxx.cfg")
	// every start: mixxx rewrites mixxx.cfg on quit, so re-assert what we need.
	for _, kv := range [][3]string{
		{"[Controller]", "Ableton_Push_3_Live_Port", "1"},
		{"[ControllerPreset]", "Ableton_Push_3_Live_Port", filepath.Join(ctl, "Ableton-Push-3.midi.xml")},
		{"[Config]", "ResizableSkin", "PushHack"},
		{"[Config]", "hide_menubar", "1"},
		{"[Config]", "show_menubar_hint", "0"},
		{"[Config]", "StartInFullscreen", "0"},
		{"[Library]", "RescanOnStartup", "1"},   // tracks copied in while push mode show up
		{"[Library]", "ShowITunesLibrary", "0"}, // no itunes on push, and its tree item hangs the library
		{"[Waveform]", "WaveformType", "25"},    // Stacked (all-shaders): rekordbox-style 3 band
		{"[Waveform]", "VisualGain_0", "1.5"},   // waveform fills its height after auto gain turns loud tracks down
		// per-band display gain (low, mid, high). mixxx draws low, then mid, then
		// high over it, each from center: shrinking mid and high keeps them inside,
		// so it reads like rekordbox: blue outside, amber inside, white core.
		{"[Waveform]", "VisualGain_1", "1.0"},
		{"[Waveform]", "VisualGain_2", "0.6"},
		{"[Waveform]", "VisualGain_3", "0.4"},
		{"[Waveform]", "OverviewNormalized", "1"},
		{"[ReplayGain]", "ReplayGainEnabled", "1"}, // auto gain
		{"[ReplayGain]", "ReplayGainAnalyserEnabled", "1"},
		// replaygain 2 aims at -18 LUFS: club tracks come out 7-12 dB down.
		// no boost (+3 / +6 were tried; user wants plain 0 dB). tune here.
		{"[ReplayGain]", "InitialReplayGainBoost", "0"},
		// isolator crossovers, pioneer DJM-like (band centers ~70 Hz, 1 kHz, 13 kHz).
		// EQ type: effects.xml, see seedEffects.
		{"[Mixer Profile]", "LoEQFrequency", "265"},
		{"[Mixer Profile]", "LoEQFrequencyPrecise", "265.000000"},
		{"[Mixer Profile]", "HiEQFrequency", "3600"},
		{"[Mixer Profile]", "HiEQFrequencyPrecise", "3600.000000"},
		{"[Waveform]", "ZoomSynchronization", "1"}, // stacked waveforms line up beat for beat
	} {
		if err := cfgSet(cfg, kv[0], kv[1], kv[2]); err != nil {
			return err
		}
	}
	if err := seedEffects(st); err != nil {
		return err
	}
	if err := writeSoundConfig(filepath.Join(st, "soundconfig.xml"), loadAudio(st)); err != nil {
		return err
	}
	if err := chownTree(musicDir, mixxxUser); err != nil {
		return err
	}
	return chownTree(st, mixxxUser)
}

func (b *bridge) startMixxx() (*exec.Cmd, error) {
	u, err := user.Lookup(mixxxUser)
	if err != nil {
		return nil, err
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	var groups []uint32
	gids, _ := u.GroupIds() // audio: needed for the sound card
	for _, g := range gids {
		n, _ := strconv.Atoi(g)
		groups = append(groups, uint32(n))
	}
	logf, err := os.OpenFile(filepath.Join(b.dir, "settings", "mixxx-stdout.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	// audio thread may go SCHED_FIFO (portaudio, PaAlsa_EnableRealtimeScheduling).
	// push3 is stopped in mixxx mode, so it competes with nothing that matters.
	if err := syscall.Setrlimit(rlimitRTPrio, &syscall.Rlimit{Cur: maxRTPrio, Max: maxRTPrio}); err != nil {
		log.Printf("rtprio: %v", err)
	}
	cmd := exec.Command(filepath.Join(b.dir, "mixxx.sh"))
	cmd.Dir = b.dir
	// USER=".": mixxx finds rekordbox drives under /run/media/$USER, but push
	// mounts them straight in /run/media/<label>-<dev>. "/run/media/." hits them.
	cmd.Env = []string{"HOME=" + u.HomeDir, "USER=.", "DISPLAY=:0", "PATH=/usr/bin:/bin",
		"MIXXX_SETTINGS=" + filepath.Join(b.dir, "settings")}
	cmd.Stdout, cmd.Stderr = logf, logf
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: groups},
		Setpgid:    true,
	}
	if err := cmd.Start(); err != nil {
		logf.Close()
		return nil, err
	}
	logf.Close() // child has its own copy
	log.Printf("mixxx started, pid %d", cmd.Process.Pid)
	return cmd, nil
}

// stopGroup: SIGTERM the process group, SIGKILL after timeout.
func stopGroup(cmd *exec.Cmd, timeout time.Duration) {
	pgid := cmd.Process.Pid
	syscall.Kill(-pgid, syscall.SIGTERM) //nolint:errcheck
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if syscall.Kill(-pgid, 0) != nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	log.Printf("mixxx did not quit in %s, killing", timeout)
	syscall.Kill(-pgid, syscall.SIGKILL) //nolint:errcheck
}

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// waitNoProc waits until no process has /proc/<pid>/comm == comm.
func waitNoProc(comm string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		found := false
		ps, _ := filepath.Glob("/proc/[0-9]*/comm")
		for _, p := range ps {
			if c, _ := os.ReadFile(p); strings.TrimSpace(string(c)) == comm {
				found = true
				break
			}
		}
		if !found {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func chownTree(root, name string) error {
	u, err := user.Lookup(name)
	if err != nil {
		return err
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	return filepath.Walk(root, func(p string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(p, uid, gid)
	})
}

// ── push-manager display calls ────────────────────────────────────────────

var httpc = &http.Client{Timeout: 5 * time.Second}

func pmDirect(on bool) error {
	resp, err := httpc.Post(pmDisplay+"direct", "application/json", strings.NewReader(fmt.Sprintf(`{"enabled":%t}`, on)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("push-manager: %s", resp.Status)
	}
	return nil
}

func pmImage(img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return postFrame(buf.Bytes())
}

// showMessage puts a plain three-line status screen up. ascii only.
func showMessage(title, line1, line2 string) {
	img := image.NewNRGBA(image.Rect(0, 0, 960, 160))
	gfx.FillRect(img, 0, 0, 960, 160, color.NRGBA{0, 0, 0, 255})
	text.DrawScaled(img, 24, 56, 3, title, color.NRGBA{255, 85, 0, 255})
	text.DrawScaled(img, 24, 100, 2, line1, color.NRGBA{255, 255, 255, 255})
	text.DrawScaled(img, 24, 140, 2, line2, color.NRGBA{150, 150, 150, 255})
	if err := pmImage(img); err != nil {
		log.Printf("screen: %v", err)
	}
}

// libraryScript makes sure mixxx's db lists musicDir. with no folder listed,
// mixxx stops at startup on a "choose music library directory" dialog
// nobody can see. prints missing (no db yet), noschema, added or ok.
const libraryScript = `
import base64, os, sqlite3, sys
db, d = sys.argv[1], sys.argv[2]
if not os.path.exists(db):
    print("missing"); sys.exit()
c = sqlite3.connect(db, timeout=5)
try:
    n = c.execute("select count(*) from directories").fetchone()[0]
except sqlite3.OperationalError:
    print("noschema"); sys.exit()

# track list columns, every start: #, artist, title, bpm, key, time (no
# album). nothing on the push can change them (no right click), so this is
# the only way. format: HeaderViewState in mixxx src/proto/headers.proto;
# columns left out stay hidden.
def varint(n):
    b = b""
    while True:
        x, n = n & 0x7F, n >> 7
        b += bytes([x | 0x80 if n else x])
        if not n:
            return b
def header(cols):
    out = b""
    for i, (name, size) in enumerate(cols):
        m = b"\x08\x00\x10" + varint(size) + b"\x18" + varint(i) + b"\x20" + varint(i) + b"\x2a" + varint(len(name)) + name.encode()
        out += b"\x0a" + varint(len(m)) + m
    return base64.b64encode(out).decode()
cols = [("artist", 190), ("title", 250), ("bpm", 60), ("key", 50), ("duration", 60)]
for model, pos in (("library", 0), ("crate", 0), ("playlist", 1), ("setlog", 1), ("rekordbox.playlistmodel", 1)):
    c.execute("insert or replace into settings(name, value) values(?, ?)",
              ("mixxx.db.model.%s.header_state_pb" % model, header([("position", 30)] * pos + cols)))
if n == 0:
    c.execute("insert into directories(directory) values(?)", (d,)); print("added")
else:
    print("ok")
c.commit()
`

func (b *bridge) libraryState() string {
	db := filepath.Join(b.dir, "settings", "mixxxdb.sqlite")
	out, err := exec.Command("python3", "-I", "-c", libraryScript, db, musicDir).Output()
	if err != nil {
		return "error: " + err.Error()
	}
	return strings.TrimSpace(string(out))
}

// ensureLibrary: on first run mixxx has to create its db before we can add
// the folder, so start it once, wait for the schema, add, stop. caller holds mu.
func (b *bridge) ensureLibrary() error {
	st := b.libraryState()
	if st == "ok" || st == "added" {
		return nil
	}
	if st != "missing" && st != "noschema" {
		return fmt.Errorf("%s", st)
	}
	showMessage("MIXXX MODE", "First start: setting up the library...", "This happens once")
	cmd, err := b.startMixxx()
	if err != nil {
		return err
	}
	defer stopGroup(cmd, 10*time.Second)
	for deadline := time.Now().Add(60 * time.Second); time.Now().Before(deadline); time.Sleep(time.Second) {
		if st = b.libraryState(); st == "added" || st == "ok" {
			log.Printf("library: %s added to mixxx db", musicDir)
			return nil
		}
	}
	return fmt.Errorf("mixxx db not ready after 60s (%s)", st)
}

// seedEffects writes effects.xml every start: reverb, echo, flanger in every
// effect unit, and the Bessel8 LV-Mix isolator EQ (full kill, like
// rekordbox's isolator mode) on all four decks. mixxx saves effects.xml only on
// clean quit, and we stop it with SIGTERM, so without this every start has
// empty effect slots. effect blocks come from the one-effect chain
// presets mixxx writes into effects/chains on first start; not there yet =
// try again next start.
func seedEffects(st string) error {
	out := filepath.Join(st, "effects.xml")
	var fx strings.Builder
	for _, n := range []string{"Reverb", "Echo", "Flanger"} {
		data, err := os.ReadFile(filepath.Join(st, "effects", "chains", n+".xml"))
		if os.IsNotExist(err) {
			return nil
		} else if err != nil {
			return err
		}
		s := string(data)
		i, j := strings.Index(s, "<Effect>"), strings.LastIndex(s, "</Effect>")
		if i < 0 || j < i {
			return fmt.Errorf("%s.xml: no <Effect>", n)
		}
		fx.WriteString(s[i : j+len("</Effect>")])
	}
	chain := "<EffectChain><Name></Name><MixMode>DRY/WET</MixMode><SuperParameterValue>0</SuperParameterValue><Effects>" +
		fx.String() + "</Effects></EffectChain>"
	eq := "<EqualizerEffects>"
	for _, g := range []string{"[Channel1]", "[Channel2]", "[Channel3]", "[Channel4]"} {
		eq += `<Id group="` + g + `">org.mixxx.effects.bessel8lvmixeq Built-In</Id>`
	}
	eq += "</EqualizerEffects>"
	doc := "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<MixxxEffects><Rack><Chains>" +
		strings.Repeat(chain, 4) + "</Chains></Rack>" + eq + "</MixxxEffects>\n"
	return os.WriteFile(out, []byte(doc), 0o644)
}
