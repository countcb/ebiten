// Copyright 2015 Hajime Hoshi
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build ignore

package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"text/template"

	"github.com/hajimehoshi/ebiten/examples/common"
	"github.com/hajimehoshi/ebiten/internal"
)

var keyboardKeys = [][]string{
	{"Esc", "1", "2", "3", "4", "5", "6", "7", "8", "9", "0", " ", " ", " ", "Del"},
	{"Tab", "Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P", " ", " ", "BS"},
	{"Ctrl", "A", "S", "D", "F", "G", "H", "J", "K", "L", " ", " ", "Enter"},
	{"Shift", "Z", "X", "C", "V", "B", "N", "M", ",", ".", " ", " "},
	{" ", "Alt", "Space", " ", " "},
	{},
	{"", "Up", ""},
	{"Left", "Down", "Right"},
}

func drawKey(t *image.NRGBA, name string, x, y, width int) {
	const height = 16
	width--
	shape := image.NewNRGBA(image.Rect(0, 0, width, height))
	p := shape.Pix
	for j := 0; j < height; j++ {
		for i := 0; i < width; i++ {
			x := (i + j*width) * 4
			switch j {
			case 0, height - 1:
				if 3 <= i && i <= width-4 {
					p[x] = 0xff
					p[x+1] = 0xff
					p[x+2] = 0xff
					p[x+3] = 0xff
				}
			case 1, height - 2:
				if i == 2 || i == width-3 {
					p[x] = 0xff
					p[x+1] = 0xff
					p[x+2] = 0xff
					p[x+3] = 0xff
				}
			case 2, height - 3:
				if i == 1 || i == width-2 {
					p[x] = 0xff
					p[x+1] = 0xff
					p[x+2] = 0xff
					p[x+3] = 0xff
				}
			default:
				if i == 0 || i == width-1 {
					p[x] = 0xff
					p[x+1] = 0xff
					p[x+2] = 0xff
					p[x+3] = 0xff
				}
			}
		}
	}
	draw.Draw(t, image.Rect(x, y, x+width, y+height), shape, image.ZP, draw.Over)
	common.ArcadeFont.DrawTextOnImage(t, name, x+4, y+5)
}

func outputKeyboardImage() (map[string]image.Rectangle, error) {
	keyMap := map[string]image.Rectangle{}
	img := image.NewNRGBA(image.Rect(0, 0, 320, 240))
	x, y := 0, 0
	for j, line := range keyboardKeys {
		x = 0
		const height = 18
		for i, key := range line {
			width := 16
			switch j {
			default:
				switch i {
				case 0:
					width = 16 + 8*(j+2)
				case len(line) - 1:
					width = 16 + 8*(j+2)
				}
			case 4:
				switch i {
				case 0:
					width = 16 + 8*(j+2)
				case 1:
					width = 16 * 2
				case 2:
					width = 16 * 5
				case 3:
					width = 16 * 2
				case 4:
					width = 16 + 8*(j+2)
				}
			case 6, 7:
				width = 16 * 3
			}
			if key != "" {
				if err := drawKey(img, key, x, y, width); err != nil {
					return nil, err
				}
				if key != " " {
					keyMap[key] = image.Rect(x, y, x+width, y+height)
				}
			}
			x += width
		}
		y += height
	}

	palette := color.Palette([]color.Color{
		color.Transparent, color.Opaque,
	})
	palettedImg := image.NewPaletted(img.Bounds(), palette)
	draw.Draw(palettedImg, palettedImg.Bounds(), img, image.ZP, draw.Src)

	f, err := os.Create("../../_resources/images/keyboard/keyboard.png")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := png.Encode(f, palettedImg); err != nil {
		return nil, err
	}
	return keyMap, nil
}

const keyRectTmpl = `{{.License}}

// DO NOT EDIT: This file is auto-generated by gen.go.

{{.BuildTags}}

package keyboard

import (
	"image"
)

var keyboardKeyRects = map[string]image.Rectangle{}

func init() {
{{range $key, $rect := .KeyRectsMap}}	keyboardKeyRects["{{$key}}"] = image.Rect({{$rect.Min.X}}, {{$rect.Min.Y}}, {{$rect.Max.X}}, {{$rect.Max.Y}})
{{end}}}

func KeyRect(name string) (image.Rectangle, bool) {
	r, ok := keyboardKeyRects[name]
	return r, ok
}`

func outputKeyRectsGo(k map[string]image.Rectangle) error {
	license, err := internal.LicenseComment()
	if err != nil {
		return err
	}
	path := "keyrects.go"

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	tmpl, err := template.New(path).Parse(keyRectTmpl)
	if err != nil {
		return err
	}
	return tmpl.Execute(f, map[string]interface{}{
		"BuildTags":   "// +build example",
		"License":     license,
		"KeyRectsMap": k,
	})
}

func main() {
	m, err := outputKeyboardImage()
	if err != nil {
		log.Fatal(err)
	}
	if err := outputKeyRectsGo(m); err != nil {
		log.Fatal(err)
	}
}
