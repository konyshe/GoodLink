//go:build windows

package ui2

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// NewFooter 创建底部归属信息和官网链接组件
func NewFooter() fyne.CanvasObject {
	// 分隔线
	separator := widget.NewSeparator()

	// 富文本标签
	versionLabel := widget.NewRichTextFromMarkdown("**@2026 GoodLink** | ")

	// 超链接
	giteeURL, _ := url.Parse("https://gitee.com/konyshe/goodlink/releases")
	link := widget.NewHyperlink("关注最新版本", giteeURL)

	// 组合为卡片样式
	footerContent := container.NewHBox(
		layout.NewSpacer(),
		versionLabel,
		widget.NewSeparator(),
		link,
		layout.NewSpacer(),
	)

	// 使用 Card 容器包裹
	footerCard := widget.NewCard("", "", footerContent)

	// 最终布局：分隔线 + 卡片
	return container.NewVBox(separator, footerCard)
}
