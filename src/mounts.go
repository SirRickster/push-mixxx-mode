package main

// mounts.go — push automounts usb sticks (vfat) with the kernel default
// iocharset=iso8859-1: a name char outside latin-1 reads as '?', the rest
// as one latin-1 byte. mixxx asks for utf-8 paths (rekordbox db, its own db)
// -> "file not found" for any non-ascii name (Dārin, ROSALÍA). in mixxx mode,
// mount them again with the utf8 option; back to push puts the old options
// back. NOT iocharset=utf8: that makes vfat case-sensitive, and rekordbox
// (mac) writes Contents/Bakey for a folder named BAKEY. vfat ignores both on
// remount, so umount + mount. polled: a stick plugged in during mixxx mode
// gets the same.

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const mediaDir = "/run/media/"

type fatMount struct{ dev, dir, opts string }

type usbWatch struct {
	stop  chan struct{}
	done  chan struct{}
	orig  map[string]string    // dir -> options push mounted it with
	fails map[string]int       // dev+dir -> failed tries; a stick just plugged in is busy a moment
	idle  map[string]time.Time // partition -> first poll it was seen unmounted
}

// remountTries: busy right after plugging in (push's usb scan, mixxx looking
// for rekordbox drives), so retry this many polls (2 s apart) before giving up.
const remountTries = 15

// startUSBWatch remounts what is plugged in now (before mixxx opens files),
// then keeps polling for new sticks.
func startUSBWatch() *usbWatch {
	w := &usbWatch{stop: make(chan struct{}), done: make(chan struct{}), orig: map[string]string{}, fails: map[string]int{}, idle: map[string]time.Time{}}
	w.pass()
	go func() {
		defer close(w.done)
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-w.stop:
				w.restore()
				return
			case <-t.C:
				w.pass()
			}
		}
	}()
	return w
}

// Stop puts every stick still mounted back to push's options.
func (w *usbWatch) Stop() {
	close(w.stop)
	<-w.done
}

func (w *usbWatch) pass() {
	w.fixAutomount()
	for _, m := range fatMounts() {
		cs := optValue(m.opts, "iocharset")
		k := m.dev + m.dir
		if utf8Mounted(m.opts) || w.fails[k] >= remountTries {
			continue
		}
		if err := remount(m, utf8Opts(m.opts)); err != nil {
			if w.fails[k]++; w.fails[k] == remountTries {
				log.Printf("usb: %s stays %s: %v", m.dir, cs, err)
			}
			continue
		}
		w.orig[m.dir] = m.opts
		log.Printf("usb: %s mounted utf8 (was %s)", m.dir, cs)
	}
}

func (w *usbWatch) restore() {
	for _, m := range fatMounts() {
		o, ok := w.orig[m.dir]
		if !ok || !utf8Mounted(m.opts) {
			continue
		}
		if err := remount(m, o); err != nil {
			log.Printf("usb: restore %s: %v", m.dir, err)
		}
	}
	clearAutomountCache(mountedSet()) // push mode needs the automounter too
}

// automountKick: a partition unmounted this long gets its "add" event again.
// longer than the automounter's own first mount, so the two never race.
const automountKick = 5 * time.Second

// fixAutomount: push's udev mount.sh skips a stick while
// /tmp/.automount-<label>-<dev> exists, and only deletes that file when the
// unplug cleanup runs, which it often doesn't here (seen: 4 stale files,
// "already cached", stick plugged in three times and never mounted). drop
// stale files, then replay "add" for a partition left unmounted.
func (w *usbWatch) fixAutomount() {
	mounted := mountedSet()
	clearAutomountCache(mounted)
	parts, _ := filepath.Glob("/sys/class/block/sd*[0-9]")
	seen := map[string]bool{}
	for _, p := range parts {
		dev := "/dev/" + filepath.Base(p)
		seen[dev] = true
		if mounted[dev] {
			delete(w.idle, dev)
			continue
		}
		t, ok := w.idle[dev]
		if !ok {
			w.idle[dev] = time.Now()
			continue
		}
		if t.IsZero() || time.Since(t) < automountKick {
			continue // kicked once already, or not long enough
		}
		w.idle[dev] = time.Time{}
		if err := run("udevadm", "trigger", "--action=add", p); err != nil {
			log.Printf("usb: %s: %v", dev, err)
		} else {
			log.Printf("usb: %s was not mounted, asked the automounter again", dev)
		}
	}
	for dev := range w.idle {
		if !seen[dev] {
			delete(w.idle, dev) // unplugged: next time it gets a fresh kick
		}
	}
}

// mountedSet: every mounted device and mount point.
func mountedSet() map[string]bool {
	data, _ := os.ReadFile("/proc/mounts")
	set := map[string]bool{}
	for _, l := range strings.Split(string(data), "\n") {
		if f := strings.Fields(l); len(f) >= 2 {
			set[f[0]] = true
			set[mountUnescape.Replace(f[1])] = true
		}
	}
	return set
}

// clearAutomountCache removes mount.sh's cache files whose mount is gone,
// and their mount point if empty.
func clearAutomountCache(mounted map[string]bool) {
	files, _ := filepath.Glob("/tmp/.automount-*")
	for _, f := range files {
		dir := mediaDir + strings.TrimPrefix(filepath.Base(f), ".automount-")
		if !mounted[dir] {
			os.Remove(f)
			os.Remove(dir) // empty only: never a mounted or full dir
		}
	}
}

// remount: umount fails (busy) -> nothing changed. mount fails -> try the old
// options so the stick doesn't vanish.
func remount(m fatMount, opts string) error {
	if err := run("umount", m.dir); err != nil {
		return err
	}
	if err := run("mount", "-t", "vfat", "-o", opts, m.dev, m.dir); err != nil {
		run("mount", "-t", "vfat", "-o", m.opts, m.dev, m.dir) //nolint:errcheck
		return err
	}
	return nil
}

func fatMounts() []fatMount {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil
	}
	return parseFatMounts(string(data))
}

// /proc/mounts escapes space, tab, newline, backslash as \ooo octal.
var mountUnescape = strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)

func parseFatMounts(s string) []fatMount {
	var out []fatMount
	for _, l := range strings.Split(s, "\n") {
		f := strings.Fields(l)
		if len(f) < 4 || f[2] != "vfat" {
			continue
		}
		dir := mountUnescape.Replace(f[1])
		if strings.HasPrefix(dir, mediaDir) {
			out = append(out, fatMount{dev: f[0], dir: dir, opts: f[3]})
		}
	}
	return out
}

func optValue(opts, key string) string {
	for _, o := range strings.Split(opts, ",") {
		if v, ok := strings.CutPrefix(o, key+"="); ok {
			return v
		}
	}
	return ""
}

// withOpt sets key=val in a mount option list, appending it if missing.
// utf8Mounted: utf8 names with case-insensitive lookups (see top).
func utf8Mounted(opts string) bool {
	return optValue(opts, "iocharset") != "utf8" && strings.Contains(","+opts+",", ",utf8,")
}

// utf8Opts: opts + utf8, iocharset=utf8 (an older bridge) back to latin-1.
func utf8Opts(opts string) string {
	if optValue(opts, "iocharset") == "utf8" {
		opts = withOpt(opts, "iocharset", "iso8859-1")
	}
	return opts + ",utf8"
}

func withOpt(opts, key, val string) string {
	parts := strings.Split(opts, ",")
	for i, o := range parts {
		if strings.HasPrefix(o, key+"=") {
			parts[i] = key + "=" + val
			return strings.Join(parts, ",")
		}
	}
	return opts + "," + key + "=" + val
}
