//go:build windows

package ui2

import (
	"encoding/json"
	"go2"
	"image/color"
	"log"

	"goodlink/config"

	_ "embed"
	_ "net/http/pprof"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/atotto/clipboard"
)

var (
	m_work_type         string
	m_btn_local         *widget.Button
	m_btn_remote        *widget.Button
	m_validated_key     *widget.Entry
	m_ui_local          *LocalUI
	m_ui_remote         *RemoteUI
	m_button_key_create *widget.Button
	m_button_key_paste  *widget.Button
	m_btn_local_bg      *canvas.Rectangle // 本地端按钮高亮背景
	m_btn_remote_bg     *canvas.Rectangle // 远程端按钮高亮背景
)

const (
	M_APP_TITLE = "GoodLink"
)

// UI样式常量
var (
	// 统一的背景颜色
	bgColorPrimary   = color.NRGBA{R: 45, G: 45, B: 55, A: 255}   // 主要背景
	bgColorSecondary = color.NRGBA{R: 40, G: 40, B: 50, A: 255}   // 次要背景
	bgColorCard      = color.NRGBA{R: 50, G: 50, B: 60, A: 255}   // 卡片背景
	separatorColor   = color.NRGBA{R: 100, G: 100, B: 100, A: 80} // 分隔线颜色
	highlightColor   = color.NRGBA{R: 70, G: 130, B: 200, A: 255} // 选中按钮高亮颜色
	// 统一的圆角半径
	cornerRadius = float32(8)
	// 统一的间距
	paddingSize = float32(12)
)

// 更新工作模式按钮样式
func updateWorkTypeButtons(selected string) {
	m_work_type = selected
	if selected == "Local" {
		m_btn_local.Importance = widget.HighImportance
		m_btn_remote.Importance = widget.MediumImportance
		// 高亮本地端按钮背景
		m_btn_local_bg.FillColor = highlightColor
		m_btn_remote_bg.FillColor = color.Transparent
	} else {
		m_btn_local.Importance = widget.MediumImportance
		m_btn_remote.Importance = widget.HighImportance
		// 高亮远程端按钮背景
		m_btn_local_bg.FillColor = color.Transparent
		m_btn_remote_bg.FillColor = highlightColor
	}
	m_btn_local.Refresh()
	m_btn_remote.Refresh()
	m_btn_local_bg.Refresh()
	m_btn_remote_bg.Refresh()
}

// 获取当前工作类型
func GetWorkType() string {
	return m_work_type
}

// 创建工作模式选择器
func createWorkTypeSelector(configInfo *config.ConfigInfo) fyne.CanvasObject {
	// 创建本地端按钮
	m_btn_local = widget.NewButtonWithIcon("  Local端  ", theme.ComputerIcon(), func() {
		updateWorkTypeButtons("Local")
	})

	// 创建远程端按钮
	m_btn_remote = widget.NewButtonWithIcon("  Remote端  ", theme.StorageIcon(), func() {
		updateWorkTypeButtons("Remote")
	})

	// 创建按钮高亮背景
	m_btn_local_bg = canvas.NewRectangle(color.Transparent)
	m_btn_local_bg.CornerRadius = cornerRadius
	m_btn_remote_bg = canvas.NewRectangle(color.Transparent)
	m_btn_remote_bg.CornerRadius = cornerRadius

	// 将按钮包装在高亮背景容器中
	localButtonContainer := container.NewStack(
		m_btn_local_bg,
		container.NewPadded(m_btn_local),
	)
	remoteButtonContainer := container.NewStack(
		m_btn_remote_bg,
		container.NewPadded(m_btn_remote),
	)

	// 根据配置设置初始状态
	if configInfo.WorkType == "" {
		configInfo.WorkType = "Local"
	}
	updateWorkTypeButtons(configInfo.WorkType)

	// 创建分隔线
	separator := canvas.NewRectangle(separatorColor)
	separator.SetMinSize(fyne.NewSize(2, 30))

	// 创建标签
	label := widget.NewRichTextFromMarkdown("**工作端侧**")

	// 组合按钮和分隔线
	buttonGroup := container.NewHBox(
		localButtonContainer,
		separator,
		remoteButtonContainer,
	)

	// 添加装饰性背景
	bg := canvas.NewRectangle(bgColorCard)
	bg.CornerRadius = cornerRadius

	buttonWithBg := container.NewStack(bg, container.NewPadded(buttonGroup))

	return container.NewBorder(nil, nil, label, nil, buttonWithBg)
}

// 创建连接密钥输入区域
func createKeyInputSection(configInfo *config.ConfigInfo) fyne.CanvasObject {
	m_validated_key = widget.NewEntry()
	m_validated_key.SetPlaceHolder("自定义16-24字节长度")
	if len(configInfo.TunKey) > 0 {
		m_validated_key.SetText(configInfo.TunKey)
	}

	// 创建密钥标签
	keyLabel := widget.NewRichTextFromMarkdown("**连接密钥**")

	// 创建带图标的输入框容器
	// keyIcon := widget.NewIcon(theme.ConfirmIcon())

	keyInputContainer := container.NewBorder(nil, nil, nil, nil, m_validated_key)

	// 创建背景
	keyBg := canvas.NewRectangle(bgColorCard)
	keyBg.CornerRadius = cornerRadius

	keyInputWithBg := container.NewStack(keyBg, container.NewPadded(keyInputContainer))

	return container.NewBorder(nil, nil, keyLabel, nil, keyInputWithBg)
}

// 创建密钥操作按钮组
func createKeyButtons() fyne.CanvasObject {
	m_button_key_create = widget.NewButtonWithIcon("生成密钥", theme.ContentAddIcon(), func() {
		m_validated_key.SetText(string(go2.RandomBytes(24)))
	})

	key_copy_button := widget.NewButtonWithIcon("复制密钥", theme.ContentCopyIcon(), func() {
		clipboard.WriteAll(m_validated_key.Text)
	})

	m_button_key_paste = widget.NewButtonWithIcon("粘贴密钥", theme.ContentPasteIcon(), func() {
		if s, err := clipboard.ReadAll(); err == nil {
			m_validated_key.SetText(s)
		}
	})

	// 统一按钮样式
	m_button_key_create.Importance = widget.MediumImportance
	key_copy_button.Importance = widget.MediumImportance
	m_button_key_paste.Importance = widget.MediumImportance

	return container.NewGridWithColumns(3, m_button_key_create, key_copy_button, m_button_key_paste)
}

func GetMainUI(myWindow *fyne.Window) *fyne.Container {
	var configInfo config.ConfigInfo
	json.Unmarshal(go2.FileReadAll("goodlink.json"), &configInfo)
	log.Println(configInfo)

	// 如果密钥为空，自动生成密钥
	if len(configInfo.TunKey) == 0 {
		configInfo.TunKey = string(go2.RandomBytes(24))
		log.Println("自动生成密钥:", configInfo.TunKey)
	}

	m_ui_local = NewLocalUI(myWindow, &configInfo)
	m_ui_remote = NewRemoteUI(myWindow, &configInfo)

	// 创建各个UI组件
	workTypeSelector := createWorkTypeSelector(&configInfo)
	keyInputSection := createKeyInputSection(&configInfo)
	keyButtons := createKeyButtons()

	m_stats_start_button = 0
	m_activity_start_button = widget.NewActivity()
	m_button_start = widget.NewButtonWithIcon("点击启动", theme.MediaPlayIcon(), start_button_click)
	m_button_start.Importance = widget.HighImportance

	// 创建启动按钮容器（带背景）
	startButtonBg := canvas.NewRectangle(bgColorCard)
	startButtonBg.CornerRadius = cornerRadius
	startButtonContainer := container.NewStack(
		startButtonBg,
		container.NewPadded(container.NewStack(m_button_start, m_activity_start_button)),
	)

	// 初始化日志标签（用于兼容，但不再显示在UI中）
	NewLogLabel("等待启动")

	// 创建主容器，添加统一的间距和布局
	mainContent := container.New(layout.NewVBoxLayout(),
		container.NewPadded(workTypeSelector),
		container.NewPadded(keyInputSection),
		container.NewPadded(keyButtons),
		container.NewPadded(NewLogList()),
		container.NewPadded(startButtonContainer),
		NewFooter(),
	)

	// 添加外层padding，确保整体有合适的边距
	return container.NewPadded(mainContent)
}
