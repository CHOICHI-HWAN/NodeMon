// SPDX-License-Identifier: GPL-3.0-or-later

// make-icons writes the dashboard's app icons into web/public (run from the repository root):
//
//	cd tray && go run ./cmd/make-icons ../web/public
package main

import (
	"fmt"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/choichi-hwan/nodemon/tray/internal/art"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: make-icons <output folder>")
	}
	// The same indigo-to-violet as the dashboard's accent color, with a mint ball on the antenna for "live".
	p := art.Params{Top: color.RGBA{0x4f, 0x46, 0xe5, 0xff}, Bottom: color.RGBA{0x8b, 0x5c, 0xf6, 0xff}, Detail: true, DotColor: color.RGBA{0x5e, 0xea, 0xd4, 0xff}}
	for name, size := range map[string]int{"apple-touch-icon.png": 180, "icon-192.png": 192, "icon-512.png": 512} {
		f, err := os.Create(filepath.Join(os.Args[1], name))
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(f, art.Render(size, p)); err != nil {
			log.Fatal(err)
		}
		f.Close()
		fmt.Println("wrote", name)
	}
}
