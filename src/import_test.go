package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportDest(t *testing.T) {
	if d, ok := importDest("/run/media/LONG-sdb1/Contents/Phil Berg/x.mp3"); !ok || d != importDir+"/Contents/Phil Berg/x.mp3" {
		t.Fatalf("dest: %q %v", d, ok)
	}
	if _, ok := importDest(musicDir + "/a.mp3"); ok {
		t.Fatal("local track must not be copied")
	}
	if got := asciiOnly("Dārin"); got != "D?rin" {
		t.Fatalf("asciiOnly: %q", got)
	}
}

// importScript against a tiny db with mixxx's columns: newest playlist +
// tracks in order, then relocate moves only pairs whose copy exists, then
// cleanup deletes the copy no track uses.
func TestImportScript(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	dir := t.TempDir()
	db := filepath.Join(dir, "mixxxdb.sqlite")
	usb := filepath.Join(dir, "usb")
	copied := filepath.Join(usb, "x", "b.mp3")
	unused := filepath.Join(usb, "y", "old.mp3")
	os.MkdirAll(filepath.Dir(copied), 0o755)
	os.MkdirAll(filepath.Dir(unused), 0o755)
	os.WriteFile(copied, []byte("x"), 0o644)
	os.WriteFile(unused, []byte("x"), 0o644)
	setup := `import sqlite3, sys
c = sqlite3.connect(sys.argv[1])
c.executescript("""
create table track_locations(id integer primary key, location text, filename text, directory text, filesize int, fs_deleted int, needs_verification int);
create table library(id integer primary key, location int, mixxx_deleted int default 0);
create table Playlists(id integer primary key, name text, position int, hidden int, date_created, date_modified, locked int);
create table PlaylistTracks(id integer primary key, playlist_id int, track_id int, position int, pl_datetime_added);
create table crate_tracks(crate_id int, track_id int, unique(crate_id, track_id));
create table cues(id integer primary key, track_id int);
create table track_analysis(id integer primary key, track_id int);
insert into track_locations values (1, '/run/media/S-sda1/a.mp3', 'a.mp3', '/run/media/S-sda1', 1, 0, 1), (2, '/run/media/S-sda1/b.mp3', 'b.mp3', '/run/media/S-sda1', 1, 0, 1);
insert into library(id, location) values (10, 1), (11, 2);
insert into Playlists values (5, 'old', 1, 0, 0, 0, 0), (6, 'history', 2, 2, 0, 0, 0), (7, 'New', 3, 0, 0, 0, 0);
insert into PlaylistTracks values (1, 7, 11, 2, 0), (2, 7, 10, 1, 0);
""")`
	if out, err := exec.Command("python3", "-I", "-c", setup, db).CombinedOutput(); err != nil {
		t.Fatalf("setup: %v %s", err, out)
	}
	out, err := importQuery(db, "after", "5")
	if err != nil {
		t.Fatal(err)
	}
	var pl struct {
		Name   string
		Tracks []string
	}
	json.Unmarshal(out, &pl)
	if pl.Name != "New" || strings.Join(pl.Tracks, ",") != "/run/media/S-sda1/a.mp3,/run/media/S-sda1/b.mp3" {
		t.Fatalf("after: %s", out)
	}
	pend := filepath.Join(dir, "import-pending.json")
	os.WriteFile(pend, []byte(`[["/run/media/S-sda1/a.mp3","`+dir+`/missing.mp3"],["/run/media/S-sda1/b.mp3","`+copied+`"]]`), 0o644)
	if out, err := importQuery(db, "relocate", pend); err != nil || strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("relocate: %s %v", out, err)
	}
	left, _ := os.ReadFile(pend)
	if !strings.Contains(string(left), "missing.mp3") || strings.Contains(string(left), copied) {
		t.Fatalf("pending left: %s", left)
	}
	out, _ = exec.Command("python3", "-I", "-c", `import sqlite3,sys; print(sqlite3.connect(sys.argv[1]).execute("select location, directory, needs_verification from track_locations where id=2").fetchone())`, db).Output()
	if !strings.Contains(string(out), copied) || !strings.Contains(string(out), ", 0)") {
		t.Fatalf("relocated row: %s", out)
	}
	// a second import added b.mp3 again at the stick path, in playlist 5
	dup := `import sqlite3, sys
c = sqlite3.connect(sys.argv[1])
c.executescript("""
insert into track_locations values (3, '/run/media/S-sda1/x/b.mp3', 'b.mp3', '/run/media/S-sda1/x', 1, 0, 1);
insert into library(id, location) values (12, 3);
insert into PlaylistTracks values (3, 5, 12, 4, 0);
insert into cues values (1, 12);
""")
c.commit()`
	if out, err := exec.Command("python3", "-I", "-c", dup, db).CombinedOutput(); err != nil {
		t.Fatalf("dup: %v %s", err, out)
	}
	if out, err := importQuery(db, "merge", "/run/media/", usb); err != nil || strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("merge: %s %v", out, err)
	}
	out, _ = exec.Command("python3", "-I", "-c", `import sqlite3,sys; c=sqlite3.connect(sys.argv[1]); print(c.execute("select track_id, position from PlaylistTracks where playlist_id=5").fetchall(), c.execute("select count(*) from library where id=12").fetchone(), c.execute("select count(*) from cues").fetchone())`, db).Output()
	if strings.TrimSpace(string(out)) != "[(11, 4)] (0,) (0,)" {
		t.Fatalf("merged: %s", out)
	}
	if out, err := importQuery(db, "cleanup", usb); err != nil || strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("cleanup: %s %v", out, err)
	}
	if _, err := os.Stat(copied); err != nil {
		t.Fatal("cleanup deleted a copy in use")
	}
	if _, err := os.Stat(filepath.Dir(unused)); !os.IsNotExist(err) {
		t.Fatal("cleanup left the unused copy or its folder")
	}
}

func TestWriteReadyMerges(t *testing.T) {
	p := filepath.Join(t.TempDir(), "import-ready.json")
	if err := writeReady(p, [][2]string{{"a", "b"}}); err != nil {
		t.Fatal(err)
	}
	if err := writeReady(p, [][2]string{{"c", "d"}}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	if got := string(data); got != `[["a","b"],["c","d"]]` {
		t.Fatalf("got %s", got)
	}
}
