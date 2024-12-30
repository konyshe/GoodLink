package theme

import (
	_ "embed"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

var (
	//go:embed fonts/simhei.ttf
	NotoSansSC []byte
)

type MyTheme struct{}

var _ fyne.Theme = (*MyTheme)(nil)

// StaticName 为 fonts 目录下的 ttf 类型的字体文件名
func (m MyTheme) Font(fyne.TextStyle) fyne.Resource {
	return &fyne.StaticResource{
		StaticName:    "simhei.ttf",
		StaticContent: NotoSansSC,
	}
}

func (*MyTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return theme.DarkTheme().Color(n, v)
}

func (*MyTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DarkTheme().Icon(n)
}

func (*MyTheme) Size(n fyne.ThemeSizeName) float32 {
	return theme.DarkTheme().Size(n)
}
