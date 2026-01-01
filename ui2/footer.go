//go:build windows

package ui2

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// NewFooter 创建底部归属信息和官网链接组件
func NewFooter() fyne.CanvasObject {
	// 分隔线
	separator := canvas.NewRectangle(separatorColor)
	separator.SetMinSize(fyne.NewSize(0, 1))

	// 富文本标签
	versionLabel := widget.NewRichTextFromMarkdown("**@2026 GoodLink**")

	// 超链接
	updateURL, _ := url.Parse("https://gitee.com/konyshe/goodlink/releases")
	updateLink := widget.NewHyperlink("升级版本", updateURL)
	// 添加图标
	updateIcon := widget.NewIcon(theme.DownloadIcon())

	// 反馈问题链接
	feedbackURL, _ := url.Parse("https://gitee.com/konyshe/goodlink/issues")
	feedbackLink := widget.NewHyperlink("反馈问题", feedbackURL)
	// 添加图标
	feedbackIcon := widget.NewIcon(theme.InfoIcon())

	// 组合为卡片样式
	footerContent := container.NewHBox(
		layout.NewSpacer(),
		versionLabel,
		container.NewHBox(updateIcon, updateLink),
		container.NewHBox(feedbackIcon, feedbackLink),
		layout.NewSpacer(),
	)

	// 创建背景
	footerBg := canvas.NewRectangle(bgColorSecondary)
	footerBg.CornerRadius = cornerRadius

	// 使用带背景的容器
	footerCard := container.NewStack(
		footerBg,
		container.NewPadded(footerContent),
	)

	// 最终布局：分隔线 + 卡片
	return container.NewVBox(
		container.NewPadded(separator),
		footerCard,
	)
}
