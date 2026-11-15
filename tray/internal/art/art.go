// SPDX-License-Identifier: GPL-3.0-or-later

// Package art draws NodeMon's mark: a small smiling satellite (solar panels, an antenna with a "live" ball) on a colored
// tile. The tray icon and the dashboard's app icons (browser tab, iPhone home screen) are all drawn from it, so they
// stay one family. Drawing in code means no image files have to be kept in sync with the program.
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
	Scale       float64    // how large the satellite is drawn around the center (0 means 1); small icons use more of the tile
	Detail      bool       // the face and the panel cells; too fine to read at menu bar size
	DotColor    color.RGBA // the antenna ball; white when left empty
}

// Everything is described on a 512-unit canvas and scaled to the requested size. The satellite is drawn in its own
// frame (u to the right, v down, origin at its center) and tilted so the panels point up to the right.
const (
	unit  = 512.0
	tilt  = 28 * math.Pi / 180
	baseY = 270.0 // a little below the middle, because the antenna adds height on top
)

type paint int

const (
	none    paint = iota
	white         // the satellite
	cut           // shows the tile's own color again: panel cells, eyes, smile
	antenna       // the ball on top
)

func inRoundRect(u, v, cu, cv, w, h, r float64) bool {
	dx := math.Max(math.Abs(u-cu)-(w/2-r), 0)
	dy := math.Max(math.Abs(v-cv)-(h/2-r), 0)
	return dx*dx+dy*dy <= r*r
}

// satellite says what is drawn at one point of its own frame.
func satellite(u, v float64, detail bool) paint {
	au := math.Abs(u)
	// The antenna: a stem and a ball.
	if math.Hypot(u, v+104) <= 17 {
		return antenna
	}
	if au <= 5 && v >= -92 && v <= -50 {
		return white
	}
	// The body.
	if inRoundRect(u, v, 0, 0, 112, 112, 32) {
		if detail {
			if math.Hypot(au-21, v+6) <= 9 { // eyes
				return cut
			}
			if d := math.Hypot(u, v-6); v >= 14 && math.Abs(d-22) <= 4.5 { // smile
				return cut
			}
		}
		return white
	}
	// The arms and the two solar panels.
	if au >= 50 && au <= 82 && math.Abs(v) <= 8 {
		return white
	}
	if au >= 82 && au <= 192 && inRoundRect(au, v, 137, 0, 110, 104, 14) {
		if detail && (math.Abs(au-137) <= 3 || math.Abs(v) <= 3) { // cells
			return cut
		}
		return white
	}
	return none
}

func inTile(x, y, radius float64) bool {
	if x < 0 || x > unit || y < 0 || y > unit {
		return false
	}
	if radius <= 0 {
		return true
	}
	dx := math.Max(math.Max(radius-x, x-(unit-radius)), 0)
	dy := math.Max(math.Max(radius-y, y-(unit-radius)), 0)
	return dx*dx+dy*dy <= radius*radius
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	l := func(x, y uint8) uint8 { return uint8(math.Round(float64(x)*(1-t) + float64(y)*t)) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 0xff}
}

// Render draws the mark at size x size pixels. Each pixel is sampled 4x4 times so the edges are smooth.
func Render(size int, p Params) *image.RGBA {
	const ss = 4
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	snow := color.RGBA{0xff, 0xff, 0xff, 0xff}
	ball := p.DotColor
	if ball.A == 0 {
		ball = snow
	}
	glyph := p.Scale
	if glyph == 0 {
		glyph = 1
	}
	radius := p.Corner * unit
	scale := unit / float64(size)
	sin, cos := math.Sincos(tilt)
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
					dx, dy := (x-unit/2)/glyph, (y-baseY)/glyph
					switch satellite(dx*cos-dy*sin, dx*sin+dy*cos, p.Detail) {
					case white:
						c = snow
					case antenna:
						c = ball
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
