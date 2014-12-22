/*
Copyright 2014 Hajime Hoshi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ebiten

import (
	"github.com/hajimehoshi/ebiten/internal"
	"github.com/hajimehoshi/ebiten/internal/opengl"
	"github.com/hajimehoshi/ebiten/internal/opengl/internal/shader"
	"image"
	"image/color"
)

type innerImage struct {
	framebuffer *opengl.Framebuffer
	texture     *opengl.Texture
}

func newInnerImage(texture *opengl.Texture) (*innerImage, error) {
	framebuffer, err := opengl.NewFramebufferFromTexture(texture)
	if err != nil {
		return nil, err
	}
	return &innerImage{framebuffer, texture}, nil
}

func (i *innerImage) size() (width, height int) {
	return i.framebuffer.Size()
}

func (i *innerImage) Clear() error {
	return i.Fill(color.Transparent)
}

func (i *innerImage) Fill(clr color.Color) error {
	if err := i.framebuffer.SetAsViewport(); err != nil {
		return err
	}
	r, g, b, a := internal.RGBA(clr)
	opengl.Clear(r, g, b, a)
	return nil
}

func (i *innerImage) drawImage(image *innerImage, parts []ImagePart, geo GeometryMatrix, color ColorMatrix) error {
	if err := i.framebuffer.SetAsViewport(); err != nil {
		return err
	}
	w, h := image.texture.Size()
	quads := textureQuads(parts, w, h)
	projectionMatrix := i.framebuffer.ProjectionMatrix()
	shader.DrawTexture(image.texture.Native(), projectionMatrix, quads, &geo, &color)
	return nil
}

func u(x float64, width int) float32 {
	return float32(x) / float32(internal.NextPowerOf2Int(width))
}

func v(y float64, height int) float32 {
	return float32(y) / float32(internal.NextPowerOf2Int(height))
}

func textureQuads(parts []ImagePart, width, height int) []shader.TextureQuad {
	quads := make([]shader.TextureQuad, 0, len(parts))
	for _, part := range parts {
		x1 := float32(part.Dst.X)
		x2 := float32(part.Dst.X + part.Dst.Width)
		y1 := float32(part.Dst.Y)
		y2 := float32(part.Dst.Y + part.Dst.Height)
		u1 := u(part.Src.X, width)
		u2 := u(part.Src.X+part.Src.Width, width)
		v1 := v(part.Src.Y, height)
		v2 := v(part.Src.Y+part.Src.Height, height)
		quad := shader.TextureQuad{x1, x2, y1, y2, u1, u2, v1, v2}
		quads = append(quads, quad)
	}
	return quads
}

type syncer interface {
	Sync(func())
}

// Image represents an image.
// The pixel format is alpha-premultiplied.
// Image implements image.Image.
type Image struct {
	syncer syncer
	inner  *innerImage
	pixels []uint8
}

// Size returns the size of the image.
func (i *Image) Size() (width, height int) {
	return i.inner.size()
}

// Clear resets the pixels of the image into 0.
func (i *Image) Clear() (err error) {
	i.pixels = nil
	i.syncer.Sync(func() {
		err = i.inner.Clear()
	})
	return
}

// Fill fills the image with a solid color.
func (i *Image) Fill(clr color.Color) (err error) {
	i.pixels = nil
	i.syncer.Sync(func() {
		err = i.inner.Fill(clr)
	})
	return
}

// DrawImage draws the given image on the receiver (i).
func (i *Image) DrawImage(image *Image, parts []ImagePart, geo GeometryMatrix, color ColorMatrix) (err error) {
	return i.drawImage(image.inner, parts, geo, color)
}

func (i *Image) drawImage(image *innerImage, parts []ImagePart, geo GeometryMatrix, color ColorMatrix) (err error) {
	i.pixels = nil
	i.syncer.Sync(func() {
		err = i.inner.drawImage(image, parts, geo, color)
	})
	return
}

// Bounds returns the bounds of the image.
func (i *Image) Bounds() image.Rectangle {
	w, h := i.inner.size()
	return image.Rect(0, 0, w, h)
}

// ColorModel returns the color model of the image.
func (i *Image) ColorModel() color.Model {
	return color.RGBAModel
}

// At returns the color of the image at (x, y).
// This method may read pixels from GPU to VRAM and be slow.
func (i *Image) At(x, y int) color.Color {
	if i.pixels == nil {
		i.syncer.Sync(func() {
			var err error
			i.pixels, err = i.inner.texture.Pixels()
			if err != nil {
				panic(err)
			}
		})
	}
	w, _ := i.inner.size()
	w = internal.NextPowerOf2Int(w)
	idx := 4*x + 4*y*w
	r, g, b, a := i.pixels[idx], i.pixels[idx+1], i.pixels[idx+2], i.pixels[idx+3]
	return color.RGBA{r, g, b, a}
}