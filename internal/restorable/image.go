// Copyright 2016 The Ebiten Authors
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

package restorable

import (
	"errors"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/internal/affine"
	"github.com/hajimehoshi/ebiten/internal/graphics"
	"github.com/hajimehoshi/ebiten/internal/opengl"
)

type drawImageHistoryItem struct {
	image    *Image
	vertices []float32
	colorm   affine.ColorM
	mode     opengl.CompositeMode
}

// Image represents an image that can be restored when GL context is lost.
type Image struct {
	image  *graphics.Image
	filter opengl.Filter

	// baseImage and baseColor are exclusive.
	basePixels       []uint8
	baseColor        color.RGBA
	drawImageHistory []*drawImageHistoryItem
	stale            bool

	volatile bool
	screen   bool
}

func NewImage(width, height int, filter opengl.Filter, volatile bool) *Image {
	return &Image{
		image:    graphics.NewImage(width, height, filter),
		filter:   filter,
		volatile: volatile,
	}
}

func NewImageFromImage(source *image.RGBA, width, height int, filter opengl.Filter) *Image {
	w2, h2 := graphics.NextPowerOf2Int(width), graphics.NextPowerOf2Int(height)
	p := make([]uint8, 4*w2*h2)
	for j := 0; j < height; j++ {
		copy(p[j*w2*4:(j+1)*w2*4], source.Pix[j*source.Stride:])
	}
	return &Image{
		image:      graphics.NewImageFromImage(source, width, height, filter),
		basePixels: p,
		filter:     filter,
	}
}

func NewScreenFramebufferImage(width, height int) *Image {
	return &Image{
		image:    graphics.NewScreenFramebufferImage(width, height),
		volatile: true,
		screen:   true,
	}
}

func (p *Image) Size() (int, int) {
	return p.image.Size()
}

func (p *Image) makeStale() {
	p.basePixels = nil
	p.baseColor = color.RGBA{}
	p.drawImageHistory = nil
	p.stale = true
}

func (p *Image) ClearIfVolatile() {
	if !p.volatile {
		return
	}
	p.basePixels = nil
	p.baseColor = color.RGBA{}
	p.drawImageHistory = nil
	p.stale = false
	if p.image == nil {
		panic("not reach")
	}
	p.image.Fill(color.RGBA{})
}

func (p *Image) Fill(clr color.RGBA) {
	p.basePixels = nil
	p.baseColor = clr
	p.drawImageHistory = nil
	p.stale = false
	p.image.Fill(clr)
}

func (p *Image) ReplacePixels(pixels []uint8) {
	p.image.ReplacePixels(pixels)
	p.basePixels = pixels
	p.baseColor = color.RGBA{}
	p.drawImageHistory = nil
	p.stale = false
}

func (p *Image) DrawImage(img *Image, vertices []float32, colorm affine.ColorM, mode opengl.CompositeMode) {
	if img.stale || img.volatile {
		p.makeStale()
	} else {
		p.appendDrawImageHistory(img, vertices, colorm, mode)
	}
	p.image.DrawImage(img.image, vertices, colorm, mode)
}

func (p *Image) appendDrawImageHistory(image *Image, vertices []float32, colorm affine.ColorM, mode opengl.CompositeMode) {
	if p.stale {
		return
	}
	// All images must be resolved and not stale each after frame.
	// So we don't have to care if image is stale or not here.
	item := &drawImageHistoryItem{
		image:    image,
		vertices: vertices,
		colorm:   colorm,
		mode:     mode,
	}
	p.drawImageHistory = append(p.drawImageHistory, item)
}

// At returns a color value at (x, y).
//
// Note that this must not be called until context is available.
// This means Pixels members must match with acutal state in VRAM.
func (p *Image) At(x, y int, context *opengl.Context) (color.RGBA, error) {
	w, h := p.image.Size()
	w2, h2 := graphics.NextPowerOf2Int(w), graphics.NextPowerOf2Int(h)
	if x < 0 || y < 0 || w2 <= x || h2 <= y {
		return color.RGBA{}, nil
	}
	if p.basePixels == nil || p.drawImageHistory != nil || p.stale {
		if err := p.readPixelsFromVRAM(p.image, context); err != nil {
			return color.RGBA{}, err
		}
	}
	idx := 4*x + 4*y*w2
	r, g, b, a := p.basePixels[idx], p.basePixels[idx+1], p.basePixels[idx+2], p.basePixels[idx+3]
	return color.RGBA{r, g, b, a}, nil
}

func (p *Image) MakeStaleIfDependingOn(target *Image) {
	if p.stale {
		return
	}
	// TODO: Performance is bad when drawImageHistory is too many.
	for _, c := range p.drawImageHistory {
		if c.image == target {
			p.makeStale()
			return
		}
	}
}

func (p *Image) readPixelsFromVRAM(image *graphics.Image, context *opengl.Context) error {
	var err error
	p.basePixels, err = image.Pixels(context)
	if err != nil {
		return err
	}
	p.baseColor = color.RGBA{}
	p.drawImageHistory = nil
	p.stale = false
	return nil
}

func (p *Image) ReadPixelsFromVRAMIfStale(context *opengl.Context) error {
	if p.volatile {
		return nil
	}
	if !p.stale {
		return nil
	}
	return p.readPixelsFromVRAM(p.image, context)
}

func (p *Image) HasDependency() bool {
	if p.stale {
		return false
	}
	return p.drawImageHistory != nil
}

// RestoreImage restores *graphics.Image from the pixels using its state.
func (p *Image) Restore(context *opengl.Context) error {
	w, h := p.image.Size()
	if p.screen {
		// The screen image should also be recreated because framebuffer might
		// be changed.
		p.image = graphics.NewScreenFramebufferImage(w, h)
		p.basePixels = nil
		p.baseColor = color.RGBA{}
		p.drawImageHistory = nil
		p.stale = false
		return nil
	}
	if p.volatile {
		p.image = graphics.NewImage(w, h, p.filter)
		p.basePixels = nil
		p.baseColor = color.RGBA{}
		p.drawImageHistory = nil
		p.stale = false
		return nil
	}
	if p.stale {
		// TODO: panic here?
		return errors.New("restorable: pixels must not be stale when restoring")
	}
	w2, h2 := graphics.NextPowerOf2Int(w), graphics.NextPowerOf2Int(h)
	img := image.NewRGBA(image.Rect(0, 0, w2, h2))
	if p.basePixels != nil {
		for j := 0; j < h; j++ {
			copy(img.Pix[j*img.Stride:], p.basePixels[j*w2*4:(j+1)*w2*4])
		}
	}
	gimg := graphics.NewImageFromImage(img, w, h, p.filter)
	if p.baseColor != (color.RGBA{}) {
		if p.basePixels != nil {
			panic("not reach")
		}
		gimg.Fill(p.baseColor)
	}
	for _, c := range p.drawImageHistory {
		// c.image.image must be already restored.
		if c.image.HasDependency() {
			panic("not reach")
		}
		gimg.DrawImage(c.image.image, c.vertices, c.colorm, c.mode)
	}
	p.image = gimg

	var err error
	p.basePixels, err = gimg.Pixels(context)
	if err != nil {
		return err
	}
	p.baseColor = color.RGBA{}
	p.drawImageHistory = nil
	p.stale = false
	return nil
}

func (p *Image) Dispose() {
	p.image.Dispose()
	p.image = nil
	p.basePixels = nil
	p.baseColor = color.RGBA{}
	p.drawImageHistory = nil
	p.stale = false
}

func (p *Image) IsInvalidated(context *opengl.Context) bool {
	return p.image.IsInvalidated(context)
}
