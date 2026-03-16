//go:build windows

package ui2

import (
	"crypto/tls"
	"encoding/json"
	"image/color"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type giteeRelease struct {
	TagName string `json:"tag_name"`
}

// checkLatestVersion 请求 Gitee API 获取最新版本号，与当前版本比较。
// 返回 (需要升级, 最新版本号)。
func checkLatestVersion(currentVersion string) (bool, string) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Get("https://gitee.com/api/v5/repos/konyshe/goodlink/releases/latest")
	if err != nil {
		log.Println("检查版本失败:", err)
		return false, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("检查版本失败, HTTP状态码:", resp.StatusCode)
		return false, ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("读取版本响应失败:", err)
		return false, ""
	}

	var release giteeRelease
	if err := json.Unmarshal(body, &release); err != nil {
		log.Println("解析版本JSON失败:", err)
		return false, ""
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if latestVersion != "" && latestVersion != currentVersion {
		return true, latestVersion
	}
	return false, latestVersion
}

var (
	upgradeHintColor = color.NRGBA{R: 255, G: 80, B: 60, A: 255}
	nat4WarnColor    = color.NRGBA{R: 240, G: 180, B: 40, A: 255}
	natOkColor       = color.NRGBA{R: 50, G: 220, B: 80, A: 255}
)

var (
	m_nat_hint_box  *fyne.Container
	m_nat_hint_icon *widget.Icon
	m_nat_hint_text *canvas.Text
)

// ShowNATHint 根据 STUN 检测结果显示 NAT 类型提示
func ShowNATHint(isNAT4 bool) {
	if m_nat_hint_box == nil {
		return
	}
	if isNAT4 {
		m_nat_hint_icon.SetResource(theme.WarningIcon())
		m_nat_hint_text.Text = "当前网络为NAT4"
		m_nat_hint_text.Color = nat4WarnColor
	} else {
		m_nat_hint_icon.SetResource(theme.ConfirmIcon())
		m_nat_hint_text.Text = "当前网络为NAT1-NAT3"
		m_nat_hint_text.Color = natOkColor
	}
	m_nat_hint_text.Refresh()
	m_nat_hint_box.Show()
}

// NewFooter 创建底部归属信息和官网链接组件
func NewFooter(currentVersion string) fyne.CanvasObject {
	separator := canvas.NewRectangle(separatorColor)
	separator.SetMinSize(fyne.NewSize(0, 1))

	//versionLabel := widget.NewRichTextFromMarkdown("**@2026 Goodlink**")

	updateURL, _ := url.Parse("https://gitee.com/konyshe/goodlink/releases")
	updateLink := widget.NewHyperlink("升级版本", updateURL)
	updateIcon := widget.NewIcon(theme.DownloadIcon())

	feedbackURL, _ := url.Parse("https://gitee.com/konyshe/goodlink/issues")
	feedbackLink := widget.NewHyperlink("反馈问题", feedbackURL)
	feedbackIcon := widget.NewIcon(theme.InfoIcon())

	newBadge := canvas.NewText("", upgradeHintColor)
	newBadge.TextSize = 14
	newBadge.TextStyle = fyne.TextStyle{Bold: true}

	upgradeBox := container.NewHBox(updateIcon, newBadge, updateLink)
	upgradeBox.Hide()

	m_nat_hint_icon = widget.NewIcon(theme.WarningIcon())
	m_nat_hint_text = canvas.NewText("", nat4WarnColor)
	m_nat_hint_text.TextSize = 14
	m_nat_hint_text.TextStyle = fyne.TextStyle{Bold: true}
	m_nat_hint_box = container.NewHBox(m_nat_hint_icon, m_nat_hint_text)
	m_nat_hint_box.Hide()

	footerContent := container.NewHBox(
		m_nat_hint_box,
		layout.NewSpacer(),
		upgradeBox,
		container.NewHBox(feedbackIcon, feedbackLink),
	)

	go func() {
		needUpgrade, latestVer := checkLatestVersion(currentVersion)
		if needUpgrade {
			fyne.Do(func() {
				updateLink.SetText("v" + latestVer)
				newBadge.Text = "有新版本!"
				newBadge.Refresh()
				upgradeBox.Show()
			})
		}
	}()

	return container.NewVBox(
		separator,
		footerContent,
	)
}
