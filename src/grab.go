package main

// grab.go — copy mixxx's 960x160 window (PushHack skin) from X :0 to the
// push screen through push-manager. a mixxx dialog (opens off that area)
// is drawn instead while it is up; the jog drives it (mapping, dialogOpen).

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/federico-pepe/ableton-push-hack/core/gfx"
	"github.com/federico-pepe/ableton-push-hack/core/gfx/text"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

const (
	grabFPS = 20
	screenW = 960
	screenH = 160
)

type grabber struct {
	stop chan struct{}
	done chan struct{}
}

func startGrab(pid int, man *manual, kb *keyboard, imp *importer) *grabber {
	g := &grabber{stop: make(chan struct{}), done: make(chan struct{})}
	go g.loop(uint32(pid), man, kb, imp)
	return g
}

func (g *grabber) Stop() {
	close(g.stop)
	<-g.done
}

func (g *grabber) loop(pid uint32, man *manual, kb *keyboard, imp *importer) {
	defer close(g.done)
	var x *xgb.Conn
	var win, dlg xproto.Window
	var last []byte
	img := image.NewNRGBA(image.Rect(0, 0, screenW, screenH))
	enc := png.Encoder{CompressionLevel: png.NoCompression} // cpu over bytes, it is localhost
	tick := time.NewTicker(time.Second / grabFPS)
	defer tick.Stop()
	defer func() {
		if x != nil {
			x.Close()
		}
	}()
	lastErr := ""
	manVer, kbVer, impVer := 0, 0, 0
	ticks := 0
	logErr := func(err error) {
		if err != nil && err.Error() != lastErr {
			log.Printf("grab: %v", err)
		}
		if err == nil {
			lastErr = ""
		} else {
			lastErr = err.Error()
		}
	}
	for {
		select {
		case <-g.stop:
			return
		case <-tick.C:
		}
		// manual open: draw it instead of the mixxx window
		if on, ver := man.State(); on || ver != manVer {
			if ver != manVer {
				manVer = ver
				last = nil // closed: repost the mixxx frame
				if on {
					logErr(pmImage(man.Frame()))
				}
			}
			if on {
				continue
			}
		}
		if x == nil {
			c, err := xgb.NewConnDisplay(":0")
			if err != nil {
				logErr(err)
				continue
			}
			x = c
		}
		if win == 0 {
			win = findWindow(x, pid) // mixxx takes a few seconds to map it
			if win == 0 {
				continue
			}
			log.Printf("grab: mixxx window 0x%x", win)
		}
		if ticks++; ticks%grabFPS == 0 { // once a second
			if d := findDialog(x, pid, win); d != dlg {
				dlg, last = d, nil
				if d != 0 {
					showDialog(x, d)
				}
			}
		}
		src, w, h := win, screenW, screenH
		if dlg != 0 {
			geo, err := xproto.GetGeometry(x, xproto.Drawable(dlg)).Reply()
			if err != nil {
				dlg, last = 0, nil // closed
				continue
			}
			src, w, h = dlg, int(geo.Width), int(geo.Height)
		}
		r, err := xproto.GetImage(x, xproto.ImageFormatZPixmap, xproto.Drawable(src), 0, 0, uint16(w), uint16(h), 0xffffffff).Reply()
		if err != nil {
			logErr(err)
			if dlg != 0 {
				dlg, last = 0, nil
			} else {
				win = 0 // gone or resized: look again
			}
			continue
		}
		kbOn, kv := kb.State()
		line, iv := imp.Line()
		if bytes.Equal(r.Data, last) && kv == kbVer && iv == impVer {
			continue
		}
		kbVer, impVer = kv, iv
		last = append(last[:0], r.Data...)
		if dlg != 0 {
			drawDialog(img, r.Data, w, h)
		} else {
			// 32 bpp BGRX -> NRGBA
			for i, o := 0, 0; i+3 < len(r.Data) && o+3 < len(img.Pix); i, o = i+4, o+4 {
				img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = r.Data[i+2], r.Data[i+1], r.Data[i], 255
			}
			if kbOn {
				kb.Draw(img)
			}
		}
		if line != "" { // import status over the bottom row
			gfx.FillRect(img, 0, screenH-14, screenW, 14, color.NRGBA{0, 0, 0, 255})
			text.Draw(img, 6, screenH-3, text.Truncate(line, 135), color.NRGBA{255, 140, 0, 255})
		}
		var buf bytes.Buffer
		enc.Encode(&buf, img) //nolint:errcheck
		logErr(postFrame(buf.Bytes()))
	}
}

// pidWindows lists the client windows owned by pid, from the window
// manager's _NET_CLIENT_LIST.
func pidWindows(x *xgb.Conn, pid uint32) []xproto.Window {
	root := xproto.Setup(x).DefaultScreen(x).Root
	list, pidAtom := atom(x, "_NET_CLIENT_LIST"), atom(x, "_NET_WM_PID")
	p, err := xproto.GetProperty(x, false, root, list, xproto.AtomWindow, 0, 1024).Reply()
	if err != nil {
		return nil
	}
	var out []xproto.Window
	for i := 0; i+3 < len(p.Value); i += 4 {
		w := xproto.Window(xgb.Get32(p.Value[i:]))
		wp, err := xproto.GetProperty(x, false, w, pidAtom, xproto.AtomCardinal, 0, 1).Reply()
		if err == nil && len(wp.Value) >= 4 && xgb.Get32(wp.Value) == pid {
			out = append(out, w)
		}
	}
	return out
}

// findWindow returns mixxx's main window: the biggest one owned by pid.
func findWindow(x *xgb.Conn, pid uint32) xproto.Window {
	var best xproto.Window
	bestArea := 0
	for _, w := range pidWindows(x, pid) {
		geo, err := xproto.GetGeometry(x, xproto.Drawable(w)).Reply()
		if err != nil || geo.Width < screenW || geo.Height < screenH {
			continue
		}
		if a := int(geo.Width) * int(geo.Height); a > bestArea {
			best, bestArea = w, a
		}
	}
	return best
}

// findDialog returns a mixxx window other than main (a dialog: error
// message, confirm), 0 if none.
func findDialog(x *xgb.Conn, pid uint32, main xproto.Window) xproto.Window {
	for _, w := range pidWindows(x, pid) {
		if w != main {
			return w
		}
	}
	return 0
}

// showDialog logs a new dialog and moves it to the screen's top left
// corner: GetImage fails on a window that is partly off the X screen.
func showDialog(x *xgb.Conn, w xproto.Window) {
	title := ""
	if p, err := xproto.GetProperty(x, false, w, atom(x, "_NET_WM_NAME"), xproto.GetPropertyTypeAny, 0, 64).Reply(); err == nil {
		title = string(p.Value)
	}
	log.Printf("grab: mixxx dialog 0x%x %q", w, title)
	xproto.ConfigureWindow(x, w, xproto.ConfigWindowX|xproto.ConfigWindowY, []uint32{0, 0})
}

// drawDialog: dialog pixels (32 bpp BGRX, w x h) scaled down to fit above a
// hint row, centered, rest black. nearest pixel: dialogs are mostly text,
// and most fit without scaling.
func drawDialog(img *image.NRGBA, data []byte, w, h int) {
	gfx.FillRect(img, 0, 0, screenW, screenH, color.NRGBA{0, 0, 0, 255})
	const hintH = 14
	s := min(1.0, float64(screenW)/float64(w), float64(screenH-hintH)/float64(h))
	dw, dh := int(float64(w)*s), int(float64(h)*s)
	x0 := (screenW - dw) / 2
	for y := 0; y < dh; y++ {
		sy := int(float64(y) / s)
		for x := 0; x < dw; x++ {
			i := (sy*w + int(float64(x)/s)) * 4
			if i+3 >= len(data) {
				continue
			}
			o := img.PixOffset(x0+x, y)
			img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = data[i+2], data[i+1], data[i], 255
		}
	}
	hint := "JOG: turn = next button, press = click it"
	text.Draw(img, (screenW-text.Width(hint))/2, screenH-3, hint, color.NRGBA{255, 140, 0, 255})
}

func atom(x *xgb.Conn, name string) xproto.Atom {
	r, err := xproto.InternAtom(x, true, uint16(len(name)), name).Reply()
	if err != nil {
		return 0
	}
	return r.Atom
}

func postFrame(png []byte) error {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("image", "frame.png")
	fw.Write(png) //nolint:errcheck
	mw.Close()
	resp, err := httpc.Post(pmDisplay+"image", mw.FormDataContentType(), &body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &httpErr{resp.Status}
	}
	return nil
}

type httpErr struct{ s string }

func (e *httpErr) Error() string { return "push-manager: " + e.s }
