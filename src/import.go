package main

// import.go — IMPORT (button 2 below the screen, library open): the mapping
// sets [Library],import_playlist and patched mixxx runs its own "Import as
// Playlist" on the selected rekordbox playlist (patches/sidebar-import-
// playlist.patch), so hot cues, grid and order come from mixxx's own
// rekordbox reader. this side sees the same press, waits for the new mixxx
// playlist, copies its files from the stick to musicDir/USB/<path on stick>,
// and records each pair in settings/import-pending.json. when the copy is
// done it drops the pairs into settings/import-ready.json: patched mixxx
// watches the settings dir and repoints the tracks right away
// (patches/import-relocate.patch). relocate() is the fallback at the next
// mixxx start, for pairs mixxx missed (a second ready file landing before it
// read the first).

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	importDir  = musicDir + "/USB"
	importWait = 120 * time.Second // mixxx reads every track's analysis first
	importShow = 6 * time.Second   // status line stays this long after the end
)

var errStopped = errors.New("stopped")

// importScript: mixxx db queries. "after N" = newest normal playlist (hidden
// 0) with id > N as json, {} if none yet. "relocate PENDING" = repoint
// copied tracks, keep pairs whose copy isn't there yet, print the count done.
// "merge MEDIA LOCAL" = a track at MEDIA/<stick>/x whose copy LOCAL/x is
// already a library track (a track in two imported playlists, added again by
// the second import) is merged into that track: its playlist places move
// over, the duplicate goes; print the count.
// "cleanup DIR" = delete files under DIR no visible library track points at
// (track removed or hidden in mixxx), and the folders left empty; print the
// count.
const importScript = `
import json, os, sqlite3, sys
db, cmd = sys.argv[1], sys.argv[2]
if cmd == "relocate":
    pend = sys.argv[3]
    try:
        pairs = json.load(open(pend))
    except FileNotFoundError:
        print(0); sys.exit()
    c = sqlite3.connect(db, timeout=10)
    left, done = [], 0
    for a, b in pairs:
        if not os.path.exists(b):
            left.append([a, b]); continue
        if c.execute("select 1 from track_locations where location=?", (b,)).fetchone():
            continue  # already known at the new place
        n = c.execute("update track_locations set location=?, directory=?, filename=?, fs_deleted=0, needs_verification=0 where location=?",
                      (b, os.path.dirname(b), os.path.basename(b), a)).rowcount
        done += n
    c.commit()
    json.dump(left, open(pend, "w"))
    print(done)
    sys.exit()
if cmd == "merge":
    media, local = sys.argv[3], sys.argv[4]
    c = sqlite3.connect(db, timeout=10)
    n = 0
    q = "select l.id, tl.id from library l join track_locations tl on tl.id=l.location where tl.location=?"
    for dup, dup_loc, loc in c.execute("""select l.id, tl.id, tl.location from library l
            join track_locations tl on tl.id=l.location where instr(tl.location, ?)=1""", (media,)).fetchall():
        rel = loc[len(media):].split("/", 1)
        keep = len(rel) == 2 and c.execute(q, (os.path.join(local, rel[1]),)).fetchone()
        if not keep or keep[0] == dup:
            continue
        c.execute("update PlaylistTracks set track_id=? where track_id=?", (keep[0], dup))
        c.execute("update or ignore crate_tracks set track_id=? where track_id=?", (keep[0], dup))
        for t in ("crate_tracks", "cues", "track_analysis"):
            c.execute(f"delete from {t} where track_id=?", (dup,))
        c.execute("delete from library where id=?", (dup,))
        c.execute("delete from track_locations where id=?", (dup_loc,))
        n += 1
    c.commit()
    print(n)
    sys.exit()
c = sqlite3.connect("file:" + db + "?mode=ro", uri=True, timeout=10)
if cmd == "max":
    print(c.execute("select coalesce(max(id), 0) from Playlists").fetchone()[0])
elif cmd == "cleanup":
    root = sys.argv[3]
    keep = {r[0] for r in c.execute("""select tl.location from track_locations tl
        join library l on l.location=tl.id where l.mixxx_deleted=0""")}
    n = 0
    for d, _, files in os.walk(root, topdown=False):
        for f in files:
            if os.path.join(d, f) not in keep:
                os.remove(os.path.join(d, f)); n += 1
        if d != root and not os.listdir(d):
            os.rmdir(d)
    print(n)
elif cmd == "after":
    r = c.execute("select id, name from Playlists where hidden=0 and id>? order by id desc limit 1", (int(sys.argv[3]),)).fetchone()
    if not r:
        print("{}"); sys.exit()
    t = [x[0] for x in c.execute("""select tl.location from PlaylistTracks pt
        join library l on l.id=pt.track_id join track_locations tl on tl.id=l.location
        where pt.playlist_id=? order by pt.position""", (r[0],))]
    print(json.dumps({"id": r[0], "name": r[1], "tracks": t}))
`

func importQuery(db string, args ...string) ([]byte, error) {
	out, err := exec.Command("python3", append([]string{"-I", "-c", importScript, db}, args...)...).Output()
	if err != nil {
		return nil, fmt.Errorf("db %s: %w", args[0], err)
	}
	return out, nil
}

type importer struct {
	st   string // settings dir: mixxxdb.sqlite, import-pending.json
	stop chan struct{}
	wg   sync.WaitGroup

	mu   sync.Mutex
	busy bool
	line string // status line on the push screen, "" = none
	ver  int
}

func newImporter(st string) *importer { return &importer{st: st, stop: make(chan struct{})} }

// Start: IMPORT was pressed with the library open. ignored while one runs.
func (im *importer) Start() {
	im.mu.Lock()
	defer im.mu.Unlock()
	if im.busy {
		return
	}
	im.busy = true
	im.wg.Add(1)
	go im.run()
}

// Stop ends a running copy between two reads (the half-copied file is
// removed) and waits for it. mixxx mode is ending.
func (im *importer) Stop() {
	close(im.stop)
	im.wg.Wait()
}

// Line: status line for the screen and its change counter.
func (im *importer) Line() (string, int) {
	im.mu.Lock()
	defer im.mu.Unlock()
	return im.line, im.ver
}

func (im *importer) say(s string) {
	im.mu.Lock()
	im.line, im.ver = asciiOnly(s), im.ver+1
	im.mu.Unlock()
}

func (im *importer) run() {
	defer im.wg.Done()
	err := im.importOne()
	switch {
	case errors.Is(err, errStopped):
		log.Printf("import: stopped")
		return
	case err != nil:
		log.Printf("import: %v", err)
		im.say("IMPORT failed: " + err.Error())
	}
	select {
	case <-time.After(importShow):
	case <-im.stop:
	}
	im.mu.Lock()
	im.busy, im.line, im.ver = false, "", im.ver+1
	im.mu.Unlock()
}

func (im *importer) importOne() error {
	db := filepath.Join(im.st, "mixxxdb.sqlite")
	out, err := importQuery(db, "max")
	if err != nil {
		return err
	}
	before := strings.TrimSpace(string(out))
	im.say("IMPORT: reading the playlist from the stick...")

	// mixxx creates the playlist, then adds the tracks: wait until the
	// track list stops growing.
	var pl struct {
		ID     int      `json:"id"`
		Name   string   `json:"name"`
		Tracks []string `json:"tracks"`
	}
	prev := -1
	for deadline := time.Now().Add(importWait); ; {
		if time.Now().After(deadline) {
			return fmt.Errorf("no playlist. Select a Rekordbox playlist in the tree first")
		}
		select {
		case <-im.stop:
			return errStopped
		case <-time.After(time.Second):
		}
		out, err := importQuery(db, "after", before)
		if err != nil {
			return err
		}
		pl.Tracks = nil
		if err := json.Unmarshal(out, &pl); err != nil {
			return err
		}
		if n := len(pl.Tracks); n > 0 && n == prev {
			break
		} else {
			prev = n
		}
	}

	type pair struct{ from, to string }
	var todo []pair
	var need int64
	for _, src := range pl.Tracks {
		dst, ok := importDest(src)
		if !ok {
			continue // already on the push
		}
		fi, err := os.Stat(src)
		if err != nil {
			continue // missing on the stick: mixxx shows it as missing too
		}
		if di, err := os.Stat(dst); err != nil || di.Size() != fi.Size() {
			need += fi.Size()
		}
		todo = append(todo, pair{src, dst})
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(musicDir, &fs); err == nil && int64(fs.Bavail)*int64(fs.Bsize) < need+1<<30 {
		return fmt.Errorf("not enough space on the Push (%d MB needed)", need>>20)
	}
	pend := filepath.Join(im.st, "import-pending.json")
	var pairs [][2]string
	if data, err := os.ReadFile(pend); err == nil {
		json.Unmarshal(data, &pairs) //nolint:errcheck // broken file: start over
	}
	var ready [][2]string
	for _, p := range todo {
		ready = append(ready, [2]string{p.from, p.to})
	}
	pairs = append(pairs, ready...)
	data, _ := json.Marshal(pairs)
	if err := os.WriteFile(pend, data, 0o644); err != nil {
		return err
	}

	log.Printf("import: %q, %d tracks, %d MB to copy", pl.Name, len(todo), need>>20)
	for i, p := range todo {
		im.say(fmt.Sprintf("IMPORT %s: %d / %d", pl.Name, i+1, len(todo)))
		if fi, err := os.Stat(p.to); err == nil {
			if si, err := os.Stat(p.from); err == nil && si.Size() == fi.Size() {
				continue // copied by an earlier import
			}
		}
		if err := copyFile(p.from, p.to, im.stop); err != nil {
			return err
		}
	}
	if err := chownTree(importDir, mixxxUser); err != nil {
		log.Printf("import: %v", err)
	}
	if err := writeReady(filepath.Join(im.st, "import-ready.json"), ready); err != nil {
		log.Printf("import: %v", err) // relocate() catches them at the next start
	}
	im.say(fmt.Sprintf("IMPORT %s: %d tracks copied, they now play from the Push.", pl.Name, len(todo)))
	log.Printf("import: %q done", pl.Name)
	return nil
}

// writeReady adds pairs to mixxx's ready file. tmp + rename: mixxx never
// reads half a file.
func writeReady(path string, pairs [][2]string) error {
	var old [][2]string
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &old) //nolint:errcheck // broken file: start over
	}
	data, _ := json.Marshal(append(old, pairs...))
	if err := os.WriteFile(path+".tmp", data, 0o644); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

// importDest: /run/media/<stick>/a/b.mp3 -> musicDir/USB/a/b.mp3. same path
// on the stick = same copy, so a track in two playlists is copied once.
func importDest(src string) (string, bool) {
	rest, ok := strings.CutPrefix(src, mediaDir)
	if !ok {
		return "", false
	}
	_, rel, ok := strings.Cut(rest, "/")
	if !ok || rel == "" {
		return "", false
	}
	return filepath.Join(importDir, rel), true
}

// copyFile copies through dst.part, so a file at dst is always complete.
func copyFile(src, dst string, stop <-chan struct{}) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	buf := make([]byte, 1<<20)
	for {
		select {
		case <-stop:
			out.Close()
			os.Remove(tmp)
			return errStopped
		default:
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				out.Close()
				os.Remove(tmp)
				return err
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			os.Remove(tmp)
			return rerr
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// relocate points mixxx at finished copies, merges duplicates a second
// import made, then deletes copies no library track uses any more. mixxx must be stopped: no copy runs, and its library
// scan would add the copies back before we look.
func relocate(st string) {
	db := filepath.Join(st, "mixxxdb.sqlite")
	out, err := importQuery(db, "relocate", filepath.Join(st, "import-pending.json"))
	if err != nil {
		log.Printf("import: %v", err)
		return // pairs not repointed yet: their copies would look unused
	}
	if n := strings.TrimSpace(string(out)); n != "0" {
		log.Printf("import: %s tracks now play from the Push", n)
	}
	if out, err = importQuery(db, "merge", mediaDir, importDir); err != nil {
		log.Printf("import: %v", err)
		return // a duplicate still at the stick keeps its copy from cleanup
	} else if n := strings.TrimSpace(string(out)); n != "0" {
		log.Printf("import: merged %s duplicate tracks", n)
	}
	if out, err = importQuery(db, "cleanup", importDir); err != nil {
		log.Printf("import: %v", err)
	} else if n := strings.TrimSpace(string(out)); n != "0" {
		log.Printf("import: deleted %s copies no longer in the library", n)
	}
}

// asciiOnly: the push font has no glyph past ascii (core/gfx/text).
func asciiOnly(s string) string {
	return strings.Map(func(r rune) rune {
		if r > 126 {
			return '?'
		}
		return r
	}, s)
}
