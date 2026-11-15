// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// colors are gray while starting, green when uploads go through, amber on a problem, red when stopped.
var colors = map[state]color.RGBA{
	stateStarting: {0x8a, 0x8f, 0x98, 0xff},
	stateOK:       {0x2e, 0x9e, 0x5b, 0xff},
	stateProblem:  {0xe0, 0xa1, 0x00, 0xff},
	stateStopped:  {0xd6, 0x45, 0x45, 0xff},
}

// makeIcon draws a round dot with a darker ring and a small white bar chart inside, as a PNG. It is made in code so
// the program needs no image files.
func makeIcon(c color.RGBA) []byte {
	const size = 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	mid := float64(size-1) / 2
	ring := color.RGBA{c.R / 2, c.G / 2, c.B / 2, 0xff}
	white := color.RGBA{0xff, 0xff, 0xff, 0xff}
	bars := [][2]int{{18, 40}, {29, 26}, {40, 32}} // x start, top y; every bar is 7 wide and ends at y=46
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			d := math.Hypot(float64(x)-mid, float64(y)-mid)
			switch {
			case d > 31:
				continue // outside the dot stays transparent
			case d > 27:
				img.SetRGBA(x, y, ring)
			default:
				img.SetRGBA(x, y, c)
			}
			for _, b := range bars {
				if x >= b[0] && x < b[0]+7 && y >= b[1] && y <= 46 {
					img.SetRGBA(x, y, white)
				}
			}
		}
	}
	var out bytes.Buffer
	_ = png.Encode(&out, img)
	return out.Bytes()
}
