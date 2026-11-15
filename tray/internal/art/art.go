// SPDX-License-Identifier: GPL-3.0-or-later

// Package art draws NodeMon's mark (three rising bars and a small "live" dot on a colored tile). The tray icon and the
// dashboard's app icons (browser tab, iPhone home screen) are all drawn from it, so they stay one family. Drawing in
// code means no image files have to be kept in sync with the program.
package art

import (
	"image"
	"image/color"
	"math"
)

// Params says how a tile looks.
type Params struct {
	Top, Bottom color.RGBA // the tile's color fades from Top (upper left) to Bottom (lower right)
	Corner      float64    // corner radius as a share of the size; 0 fills the whole square (what iOS wants, it rounds the corners itself)
	Scale       float64    // how large the bars are drawn around the center (0 means 1); small icons use more of the tile
	Dot         bool       // the live dot above the bars
	DotColor    color.RGBA
}

// Everything is described on a 512-unit canvas and scaled to the requested size.
const unit = 512.0

var bars = [][4]float64{ // x, top, width, bottom; the ends are rounded
	{120, 252, 64, 372},
	{224, 140, 64, 372},
	{328, 202, 64, 372},
}

func inRoundBar(x, y float64, b [4]float64) bool {
	r := b[2] / 2
	cx := b[0] + r
	switch {
	case y < b[1] || y > b[3] || x < b[0] || x > b[0]+b[2]:
		return false
	case y < b[1]+r:
		return math.Hypot(x-cx, y-(b[1]+r)) <= r
	case y > b[3]-r:
		return math.Hypot(x-cx, y-(b[3]-r)) <= r
	}
	return true
}

func inTile(x, y, radius float64) bool {
	if radius <= 0 {
		return x >= 0 && x <= unit && y >= 0 && y <= unit
	}
	dx := math.Max(math.Max(radius-x, x-(unit-radius)), 0)
	dy := math.Max(math.Max(radius-y, y-(unit-radius)), 0)
	return dx*dx+dy*dy <= radius*radius && x >= 0 && x <= unit && y >= 0 && y <= unit
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	l := func(x, y uint8) uint8 { return uint8(math.Round(float64(x)*(1-t) + float64(y)*t)) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 0xff}
}

// Render draws the mark at size x size pixels. Each pixel is sampled 4x4 times so the edges are smooth.
func Render(size int, p Params) *image.RGBA {
	const ss = 4
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	white := color.RGBA{0xff, 0xff, 0xff, 0xff}
	radius := p.Corner * unit
	scale := unit / float64(size)
	glyph := p.Scale
	if glyph == 0 {
		glyph = 1
	}
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var r, g, b, cover float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/ss) * scale
					y := (float64(py) + (float64(sy)+0.5)/ss) * scale
					if !inTile(x, y, radius) {
						continue
					}
					c := mix(p.Top, p.Bottom, math.Min(1, math.Max(0, (x+y)/(2*unit))))
					gx, gy := unit/2+(x-unit/2)/glyph, unit/2+(y-unit/2)/glyph // the bars live on the unscaled canvas
					for _, bar := range bars {
						if inRoundBar(gx, gy, bar) {
							c = white
							break
						}
					}
					if p.Dot && math.Hypot(gx-404, gy-134) <= 32 {
						c = p.DotColor
					}
					r, g, b, cover = r+float64(c.R), g+float64(c.G), b+float64(c.B), cover+1
				}
			}
			if cover == 0 {
				continue
			}
			// Straight (non-premultiplied) color averaged over the covered samples, alpha from the coverage.
			img.SetRGBA(px, py, color.RGBA{uint8(r / cover), uint8(g / cover), uint8(b / cover), uint8(math.Round(255 * cover / (ss * ss)))})
		}
	}
	return img
}
