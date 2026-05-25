package render

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"

	"github.com/philipbankier/technical-visualizer/internal/model"
)

const (
	fallbackWidth  = 1600
	fallbackHeight = 900
)

func WriteFallbackPNG(path string, packet model.VisualPacket) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	img := image.NewRGBA(image.Rect(0, 0, fallbackWidth, fallbackHeight))
	fill(img, img.Bounds(), color.RGBA{R: 14, G: 18, B: 22, A: 255})

	drawBand(img, image.Rect(0, 0, fallbackWidth, 130), color.RGBA{R: 24, G: 38, B: 48, A: 255})
	drawBand(img, image.Rect(0, 130, fallbackWidth, 136), color.RGBA{R: 82, G: 178, B: 255, A: 255})
	drawBand(img, image.Rect(0, 820, fallbackWidth, fallbackHeight), color.RGBA{R: 22, G: 30, B: 36, A: 255})

	drawPanel(img, image.Rect(70, 190, 690, 710), color.RGBA{R: 31, G: 43, B: 50, A: 255}, color.RGBA{R: 96, G: 207, B: 176, A: 255})
	drawPanel(img, image.Rect(760, 190, 1530, 380), color.RGBA{R: 34, G: 39, B: 52, A: 255}, color.RGBA{R: 255, G: 191, B: 105, A: 255})
	drawPanel(img, image.Rect(760, 430, 1530, 710), color.RGBA{R: 33, G: 44, B: 44, A: 255}, color.RGBA{R: 255, G: 106, B: 136, A: 255})

	counts := []int{
		len(packet.RankedClaims),
		len(packet.Facts),
		len(packet.Risks),
		len(packet.Tradeoffs),
		len(packet.Unknowns),
		len(packet.SourceRefs),
	}
	for i, count := range counts {
		x := 110 + i*95
		height := 42 + min(count, 8)*26
		drawBand(img, image.Rect(x, 650-height, x+52, 650), color.RGBA{R: uint8(92 + i*18), G: uint8(168 - i*11), B: uint8(220 - i*16), A: 255})
	}

	for i := 0; i < 5; i++ {
		y := 230 + i*76
		drawBand(img, image.Rect(805, y, 1450-(i*38), y+18), color.RGBA{R: 92, G: 118, B: 132, A: 255})
		drawBand(img, image.Rect(805, y+32, 1290-(i*31), y+43), color.RGBA{R: 54, G: 70, B: 80, A: 255})
	}

	for i := 0; i < 4; i++ {
		x := 805 + i*170
		drawPanel(img, image.Rect(x, 505, x+130, 640), color.RGBA{R: 42, G: 57, B: 58, A: 255}, color.RGBA{R: 96, G: 207, B: 176, A: 255})
	}

	// #nosec G304 -- path is the caller-selected bundle output file.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}

func drawPanel(img draw.Image, rect image.Rectangle, bg color.Color, accent color.Color) {
	fill(img, rect, bg)
	drawBand(img, image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+8, rect.Max.Y), accent)
	drawBand(img, image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+2), color.RGBA{R: 92, G: 108, B: 116, A: 255})
}

func drawBand(img draw.Image, rect image.Rectangle, c color.Color) {
	fill(img, rect, c)
}

func fill(img draw.Image, rect image.Rectangle, c color.Color) {
	draw.Draw(img, rect, &image.Uniform{C: c}, image.Point{}, draw.Src)
}
