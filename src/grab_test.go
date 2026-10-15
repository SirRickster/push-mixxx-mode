package main

import (
	"image"
	"testing"
)

// drawDialog: big dialog scales down inside the frame, small one is copied
// 1:1 and centered. no index out of range either way.
func TestDrawDialog(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, screenW, screenH))
	for _, sz := range [][2]int{{1920, 600}, {961, 1}, {300, 100}} {
		w, h := sz[0], sz[1]
		data := make([]byte, w*h*4)
		for i := range data {
			data[i] = 200
		}
		drawDialog(img, data, w, h)
	}
	// 300x100 is centered: x 330..629, rows 0..99
	if p := img.NRGBAAt(330, 50); p.R != 200 {
		t.Fatalf("dialog pixel not copied: %v", p)
	}
	if p := img.NRGBAAt(329, 50); p.R != 0 {
		t.Fatalf("left of dialog not black: %v", p)
	}
}
