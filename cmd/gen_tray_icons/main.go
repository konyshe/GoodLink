// gen_tray_icons reads assert/favicon.png and generates 4 tray state .ico files
// into assert/ using the same dot-drawing logic as ui2/tray.go.
// Run from project root: go run ./cmd/gen_tray_icons
package main

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"path/filepath"

	"github.com/antoinefink/golang-ico"
	"github.com/nfnt/resize"
)

var (
	dotColorIdle    = color.NRGBA{R: 50, G: 120, B: 240, A: 255}
	dotColorWarning = color.NRGBA{R: 240, G: 180, B: 40, A: 255}
	dotColorDanger  = color.NRGBA{R: 230, G: 60, B: 60, A: 255}
	dotColorSuccess = color.NRGBA{R: 50, G: 220, B: 80, A: 255}
)

func main() {
	basePath := "assert/favicon.png"
	if len(os.Args) > 1 {
		basePath = os.Args[1]
	}
	outDir := "assert"
	if len(os.Args) > 2 {
		outDir = os.Args[2]
	}

	f, err := os.Open(basePath)
	if err != nil {
		panic("open base image: " + err.Error())
	}
	defer f.Close()
	baseImage, _, err := image.Decode(f)
	if err != nil {
		panic("decode base image: " + err.Error())
	}
	// Resize to 32x32 for tray icon (ICO max 256x256; small size suits system tray)
	const traySize = 32
	baseImage = resize.Resize(traySize, traySize, baseImage, resize.Lanczos3)

	states := []struct {
		name string
		c    color.NRGBA
	}{
		{"tray_idle.ico", dotColorIdle},
		{"tray_warning.ico", dotColorWarning},
		{"tray_danger.ico", dotColorDanger},
		{"tray_success.ico", dotColorSuccess},
	}

	for _, s := range states {
		dst := buildIconWithDot(baseImage, s.c)
		outPath := filepath.Join(outDir, s.name)
		out, err := os.Create(outPath)
		if err != nil {
			panic("create " + outPath + ": " + err.Error())
		}
		if err := ico.Encode(out, dst); err != nil {
			_ = out.Close()
			panic("encode " + outPath + ": " + err.Error())
		}
		if err := out.Close(); err != nil {
			panic("close " + outPath + ": " + err.Error())
		}
	}
}

func buildIconWithDot(baseImage image.Image, dotColor color.NRGBA) *image.RGBA {
	bounds := baseImage.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, baseImage, bounds.Min, draw.Src)

	dotRadius := float64(w) / 4
	borderWidth := math.Max(3, dotRadius/3)
	shadowWidth := math.Max(2, borderWidth/2)

	cx := float64(w) - dotRadius - borderWidth
	cy := float64(h) - dotRadius - borderWidth

	shadowR := dotRadius + borderWidth + shadowWidth
	borderR := dotRadius + borderWidth

	shadowColor := color.NRGBA{R: 0, G: 0, B: 0, A: 100}
	borderColor := color.NRGBA{R: 255, G: 255, B: 255, A: 255}

	fillCenter := dotColor
	fillEdge := color.NRGBA{
		R: uint8(math.Max(0, float64(dotColor.R)*0.7)),
		G: uint8(math.Max(0, float64(dotColor.G)*0.7)),
		B: uint8(math.Max(0, float64(dotColor.B)*0.7)),
		A: dotColor.A,
	}

	scan := int(shadowR + 2)
	for py := int(cy) - scan; py <= int(cy)+scan; py++ {
		for px := int(cx) - scan; px <= int(cx)+scan; px++ {
			if px < 0 || px >= w || py < 0 || py >= h {
				continue
			}
			dx := float64(px) - cx
			dy := float64(py) - cy
			dist := math.Sqrt(dx*dx + dy*dy)

			if dist > shadowR+0.5 {
				continue
			}

			var c color.NRGBA
			var alpha float64

			if dist <= dotRadius-0.5 {
				t := dist / dotRadius
				c = lerpColor(fillCenter, fillEdge, t)
				alpha = 1
			} else if dist <= dotRadius+0.5 {
				t := dist / dotRadius
				c = lerpColor(fillCenter, fillEdge, t)
				alpha = 1 - (dist - (dotRadius - 0.5))
			} else if dist <= borderR-0.5 {
				c = borderColor
				alpha = 1
			} else if dist <= borderR+0.5 {
				edge := borderR - 0.5
				c = borderColor
				alpha = 1 - (dist - edge)
			} else if dist <= shadowR-0.5 {
				c = shadowColor
				alpha = 1
			} else {
				edge := shadowR - 0.5
				c = shadowColor
				alpha = 1 - (dist - edge)
			}

			if alpha <= 0 {
				continue
			}

			bg := dst.RGBAAt(px, py)
			blended := blendOver(bg, c, alpha)
			dst.SetRGBA(px, py, blended)
		}
	}

	return dst
}

func lerpColor(a, b color.NRGBA, t float64) color.NRGBA {
	t = math.Max(0, math.Min(1, t))
	return color.NRGBA{
		R: uint8(float64(a.R)*(1-t) + float64(b.R)*t),
		G: uint8(float64(a.G)*(1-t) + float64(b.G)*t),
		B: uint8(float64(a.B)*(1-t) + float64(b.B)*t),
		A: uint8(float64(a.A)*(1-t) + float64(b.A)*t),
	}
}

func blendOver(bg color.RGBA, fg color.NRGBA, fgAlpha float64) color.RGBA {
	a := float64(fg.A) / 255 * fgAlpha
	invA := 1 - a
	return color.RGBA{
		R: uint8(float64(fg.R)*a + float64(bg.R)*invA),
		G: uint8(float64(fg.G)*a + float64(bg.G)*invA),
		B: uint8(float64(fg.B)*a + float64(bg.B)*invA),
		A: uint8(math.Min(255, float64(bg.A)+a*255)),
	}
}
