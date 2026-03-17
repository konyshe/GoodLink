// One-off tool: reads assert/favicon.png and writes assert/icon_idle.png,
// icon_warning.png, icon_danger.png, icon_success.png (base + colored dot).
// Run from repo root: go run ./cmd/gen_tray_icons

package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// Same colors as ui2/start.go DotColor*
var (
	dotColorIdle    = color.NRGBA{R: 50, G: 120, B: 240, A: 255}
	dotColorWarning = color.NRGBA{R: 240, G: 180, B: 40, A: 255}
	dotColorDanger  = color.NRGBA{R: 230, G: 60, B: 60, A: 255}
	dotColorSuccess = color.NRGBA{R: 50, G: 220, B: 80, A: 255}
)

func main() {
	icoPath := "assert/favicon.ico"
	if len(os.Args) > 1 {
		icoPath = os.Args[1]
	}
	data, err := os.ReadFile(icoPath)
	if err != nil {
		panic("read base image: " + err.Error())
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic("decode base image: " + err.Error())
	}

	outDir := filepath.Dir(icoPath)
	for name, dotColor := range map[string]color.NRGBA{
		"icon_idle.png":    dotColorIdle,
		"icon_warning.png": dotColorWarning,
		"icon_danger.png":  dotColorDanger,
		"icon_success.png": dotColorSuccess,
	} {
		pngBytes := buildIconWithDot(img, dotColor)
		outPath := filepath.Join(outDir, name)
		if err := os.WriteFile(outPath, pngBytes, 0644); err != nil {
			panic("write " + outPath + ": " + err.Error())
		}
	}
}

func buildIconWithDot(baseImage image.Image, dotColor color.NRGBA) []byte {
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
				c = borderColor
				alpha = 1 - (dist - (borderR - 0.5))
			} else if dist <= shadowR-0.5 {
				c = shadowColor
				alpha = 1
			} else {
				c = shadowColor
				alpha = 1 - (dist - (shadowR - 0.5))
			}
			if alpha <= 0 {
				continue
			}
			bg := dst.RGBAAt(px, py)
			blended := blendOver(bg, c, alpha)
			dst.SetRGBA(px, py, blended)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		panic(err)
	}
	return buf.Bytes()
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
