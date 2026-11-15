// SPDX-License-Identifier: GPL-3.0-or-later
package art

import (
	"image/color"
	"testing"
)

func TestRender(t *testing.T) {
	p := Params{Top: color.RGBA{0x4f, 0x46, 0xe5, 0xff}, Bottom: color.RGBA{0x8b, 0x5c, 0xf6, 0xff}, Corner: 0.22, Dot: true, DotColor: color.RGBA{0x5e, 0xea, 0xd4, 0xff}}
	img := Render(128, p)
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Error("the rounded corner must be transparent")
	}
	if _, _, _, a := img.At(64, 120).RGBA(); a == 0 {
		t.Error("the tile must be filled")
	}
	if r, g, b, _ := img.At(64, 70).RGBA(); r>>8 < 250 || g>>8 < 250 || b>>8 < 250 { // inside the tallest bar
		t.Errorf("the bar must be white, got %d %d %d", r>>8, g>>8, b>>8)
	}
	full := Render(64, Params{Top: p.Top, Bottom: p.Bottom})
	if _, _, _, a := full.At(0, 0).RGBA(); a>>8 != 255 {
		t.Error("with no corner radius the corner is filled")
	}
}
