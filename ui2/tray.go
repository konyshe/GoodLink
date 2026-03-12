//go:build windows

package ui2

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

var (
	trayApp      desktop.App
	baseImage    image.Image
	currentDotColor color.NRGBA
)

func InitTrayIcons(iconPNG []byte) {
	img, _, err := image.Decode(bytes.NewReader(iconPNG))
	if err != nil {
		return
	}
	baseImage = img
}

func buildIconWithDot(dotColor color.NRGBA) fyne.Resource {
	if baseImage == nil {
		return nil
	}

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
				inner := dotRadius - 0.5
				t := dist / dotRadius
				c = lerpColor(fillCenter, fillEdge, t)
				alpha = 1 - (dist - inner)
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

	var buf bytes.Buffer
	png.Encode(&buf, dst)
	return fyne.NewStaticResource("tray_icon.png", buf.Bytes())
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

func SetTrayApp(desk desktop.App) {
	trayApp = desk
	if baseImage != nil {
		icon := buildIconWithDot(DotColorIdle)
		if icon != nil {
			trayApp.SetSystemTrayIcon(icon)
			currentDotColor = DotColorIdle
		}
	}
}

func UpdateTrayIcon(dotColor color.NRGBA) {
	if trayApp == nil || baseImage == nil {
		return
	}
	if dotColor == currentDotColor {
		return
	}
	currentDotColor = dotColor
	icon := buildIconWithDot(dotColor)
	if icon != nil {
		trayApp.SetSystemTrayIcon(icon)
	}
}
