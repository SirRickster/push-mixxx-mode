package main

import "testing"

func TestFatMounts(t *testing.T) {
	in := "/dev/sda1 /run/media/LONGW2-sda1 vfat rw,relatime,gid=6,codepage=437,iocharset=iso8859-1,shortname=mixed 0 0\n" +
		"/dev/sdb1 /run/media/MY\\040STICK-sdb1 vfat rw,gid=6 0 0\n" +
		"/dev/sdc1 /run/media/EX-sdc1 exfat rw,iocharset=utf8 0 0\n" +
		"/dev/mmcblk0p5 /data ext4 rw 0 0\n"
	m := parseFatMounts(in)
	if len(m) != 2 || m[0].dir != "/run/media/LONGW2-sda1" || m[1].dir != "/run/media/MY STICK-sdb1" {
		t.Fatalf("parse: %+v", m)
	}
	if got := optValue(m[0].opts, "iocharset"); got != "iso8859-1" {
		t.Fatalf("optValue: %q", got)
	}
	if got := withOpt(m[0].opts, "iocharset", "utf8"); got != "rw,relatime,gid=6,codepage=437,iocharset=utf8,shortname=mixed" {
		t.Fatalf("withOpt replace: %q", got)
	}
	if got := withOpt(m[1].opts, "iocharset", "utf8"); got != "rw,gid=6,iocharset=utf8" {
		t.Fatalf("withOpt append: %q", got)
	}
	// utf8 option, never iocharset=utf8: that one is case-sensitive
	if got := utf8Opts(m[0].opts); got != m[0].opts+",utf8" || !utf8Mounted(got) {
		t.Fatalf("utf8Opts: %q", got)
	}
	if got := utf8Opts("rw,iocharset=utf8"); got != "rw,iocharset=iso8859-1,utf8" || !utf8Mounted(got) {
		t.Fatalf("utf8Opts from iocharset=utf8: %q", got)
	}
	if utf8Mounted("rw,iocharset=utf8") || utf8Mounted(m[0].opts) {
		t.Fatal("utf8Mounted: false positive")
	}
}
