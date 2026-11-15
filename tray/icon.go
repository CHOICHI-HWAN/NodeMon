// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"bytes"
	"image/color"
	"image/png"

	"github.com/choichi-hwan/nodemon/tray/internal/art"
)

// colors are gray while starting, green when uploads go through, amber on a problem, red when stopped.
var colors = map[state]color.RGBA{
	stateStarting: {0x8a, 0x8f, 0x98, 0xff},
	stateOK:       {0x2e, 0x9e, 0x5b, 0xff},
	stateProblem:  {0xe0, 0xa1, 0x00, 0xff},
	stateStopped:  {0xd6, 0x45, 0x45, 0xff},
}

// makeIcon draws NodeMon's mark (three rising bars) on a rounded tile in the state's color, as a PNG.
func makeIcon(c color.RGBA) []byte {
	lighten := func(v uint8) uint8 { return v + (255-v)/5 }
	img := art.Render(64, art.Params{Top: color.RGBA{lighten(c.R), lighten(c.G), lighten(c.B), 0xff}, Bottom: c, Corner: 0.22, Scale: 1.12})
	var out bytes.Buffer
	_ = png.Encode(&out, img)
	return out.Bytes()
}
