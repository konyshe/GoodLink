//go:build windows

package ui2

import (
	"encoding/json"
	"go2"
	"image/color"
	"log"
	"net"
	"time"

	"goodlink/config"
	"goodlink/pro"
	"goodlink/stun2"

	_ "embed"
	_ "net/http/pprof"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/atotto/clipboard"
)

const (
	goodlinkFileName = "goodlink.json"
)

// entryWrapper 包装 Entry 以实现 uiComponent 接口
type entryWrapper struct {
	entry *widget.Entry
}

func (e *entryWrapper) Enable()  { e.entry.Enable() }
func (e *entryWrapper) Disable() { e.entry.Disable() }

// buttonWrapper 包装 Button 以实现 uiComponent 接口
type buttonWrapper struct {
	btn *widget.Button
}

func (b *buttonWrapper) Enable()  { b.btn.Enable() }
func (b *buttonWrapper) Disable() { b.btn.Disable() }

var (
	m_work_type         string
	m_btn_local         *widget.Button
	m_btn_remote        *widget.Button
	m_validated_key     *widget.Entry
	m_button_key_create *widget.Button
	m_button_key_paste  *widget.Button
	m_btn_local_bg      *canvas.Rectangle // 本地端按钮高亮背景
	m_btn_remote_bg     *canvas.Rectangle // 远程端按钮高亮背景
	m_btn_local_border  *canvas.Rectangle // 本地端按钮边框容器
	m_btn_remote_border *canvas.Rectangle // 远程端按钮边框容器
)

const (
	M_APP_TITLE    = "Goodlink"
	workTypeLocal  = "Local"
	workTypeRemote = "Remote"
)

// UI样式常量
var (
	// 统一的背景颜色
	bgColorPrimary       = color.NRGBA{R: 45, G: 45, B: 55, A: 255}   // 主要背景
	bgColorSecondary     = color.NRGBA{R: 40, G: 40, B: 50, A: 255}   // 次要背景
	bgColorCard          = color.NRGBA{R: 50, G: 50, B: 60, A: 255}   // 卡片背景
	separatorColor       = color.NRGBA{R: 100, G: 100, B: 100, A: 80} // 分隔线颜色
	highlightColor       = color.NRGBA{R: 0, G: 120, B: 255, A: 255}  // 选中按钮高亮颜色（完全不透明，更亮）
	highlightBorderColor = color.NRGBA{R: 0, G: 180, B: 255, A: 255}  // 高亮边框颜色
	// 统一的圆角半径
	cornerRadius = float32(8)
	// 统一的间距
	paddingSize = float32(12)
)

// updateButtonHighlight 更新单个按钮的高亮状态
func updateButtonHighlight(btn *widget.Button, bg *canvas.Rectangle, border *canvas.Rectangle, isHighlighted bool) {
	if isHighlighted {
		btn.Importance = widget.HighImportance
		bg.FillColor = highlightColor
		border.StrokeColor = highlightBorderColor
		border.StrokeWidth = 3
		border.FillColor = color.Transparent
	} else {
		btn.Importance = widget.MediumImportance
		bg.FillColor = color.Transparent
		border.StrokeColor = color.Transparent
		border.StrokeWidth = 0
		border.FillColor = color.Transparent
	}
	btn.Refresh()
	bg.Refresh()
	border.Refresh()
}

// 更新工作模式按钮样式
func updateWorkTypeButtons(selected string) {
	m_work_type = selected
	isLocal := selected == workTypeLocal

	updateButtonHighlight(m_btn_local, m_btn_local_bg, m_btn_local_border, isLocal)
	updateButtonHighlight(m_btn_remote, m_btn_remote_bg, m_btn_remote_border, !isLocal)
}

// 获取当前工作类型
func GetWorkType() string {
	return m_work_type
}

// createButtonWithHighlight 创建带高亮效果的按钮容器
func createButtonWithHighlight(btn *widget.Button, bg *canvas.Rectangle, border *canvas.Rectangle) *fyne.Container {
	bg.CornerRadius = cornerRadius
	border.CornerRadius = cornerRadius

	buttonInner := container.NewStack(bg, btn)
	buttonWithPadding := container.NewPadded(buttonInner)
	return container.NewStack(border, buttonWithPadding)
}

// 创建工作模式选择器
func createWorkTypeSelector(configInfo *config.ConfigInfo) fyne.CanvasObject {
	// 创建本地端按钮
	m_btn_local = widget.NewButtonWithIcon("  Local端  ", theme.ComputerIcon(), nil)

	// 创建远程端按钮
	m_btn_remote = widget.NewButtonWithIcon("  Remote端  ", theme.StorageIcon(), nil)

	// 创建按钮高亮背景
	m_btn_local_bg = canvas.NewRectangle(color.Transparent)
	m_btn_remote_bg = canvas.NewRectangle(color.Transparent)

	// 创建外层边框容器（用于更明显的高亮显示）
	m_btn_local_border = canvas.NewRectangle(color.Transparent)
	m_btn_remote_border = canvas.NewRectangle(color.Transparent)

	// 创建带高亮的按钮容器
	localButtonContainer := createButtonWithHighlight(m_btn_local, m_btn_local_bg, m_btn_local_border)
	remoteButtonContainer := createButtonWithHighlight(m_btn_remote, m_btn_remote_bg, m_btn_remote_border)

	// 根据配置设置初始状态
	if configInfo.WorkType == "" {
		configInfo.WorkType = workTypeLocal
	}
	updateWorkTypeButtons(configInfo.WorkType)

	// 创建分隔线
	separator := canvas.NewRectangle(separatorColor)
	separator.SetMinSize(fyne.NewSize(2, 30))

	// 创建标签
	label := widget.NewRichTextFromMarkdown("**工作端侧**: ")

	// 组合按钮和分隔线
	buttonGroup := container.NewHBox(
		localButtonContainer,
		separator,
		remoteButtonContainer,
	)

	return container.NewBorder(nil, nil, label, nil, buttonGroup)
}

// 创建连接密钥输入区域
func createKeyInputSection(configInfo *config.ConfigInfo) fyne.CanvasObject {
	m_validated_key = widget.NewEntry()
	m_validated_key.SetPlaceHolder("自定义16-24字节长度")
	if len(configInfo.TunKey) > 0 {
		m_validated_key.SetText(configInfo.TunKey)
	}

	// 创建密钥标签和图标
	keyLabel := widget.NewRichTextFromMarkdown("**连接密钥**: ")

	// 创建输入框容器，将标签和输入框放在同一行
	keyInputContainer := container.NewBorder(nil, nil, keyLabel, nil, m_validated_key)

	return keyInputContainer
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

	// 创建按钮容器，使用网格布局
	buttonGrid := container.NewGridWithColumns(3, m_button_key_create, key_copy_button, m_button_key_paste)

	return buttonGrid
}

func GetMainUI(myWindow *fyne.Window) *fyne.Container {
	var configInfo config.ConfigInfo
	json.Unmarshal(go2.FileReadAll(goodlinkFileName), &configInfo)
	log.Println(configInfo)

	// 如果密钥为空，自动生成密钥
	if len(configInfo.TunKey) == 0 {
		configInfo.TunKey = string(go2.RandomBytes(24))
		log.Println("自动生成密钥:", configInfo.TunKey)
	}

	// 创建各个UI组件
	workTypeSelector := createWorkTypeSelector(&configInfo)
	keyInputSection := createKeyInputSection(&configInfo)
	keyButtons := createKeyButtons()

	// 设置需要控制的UI组件列表（必须在所有组件创建后设置）
	setUIComponents([]uiComponent{
		&entryWrapper{entry: m_validated_key},
		&buttonWrapper{btn: m_button_key_create},
		&buttonWrapper{btn: m_button_key_paste},
		&buttonWrapper{btn: m_btn_local},
		&buttonWrapper{btn: m_btn_remote},
	})

	m_stats_start_button = 0
	m_activity_start_button = widget.NewActivity()
	m_button_start = widget.NewButtonWithIcon("点击启动", theme.MediaPlayIcon(), start_button_click)
	m_button_start.Importance = widget.HighImportance

	// 创建启动按钮容器
	startButtonContainer := container.NewStack(m_button_start, m_activity_start_button)

	// 创建配置区域容器（根据工作模式动态显示）
	configContainer := container.NewVBox()

	// 根据工作模式显示对应的配置
	updateConfigDisplay := func() {
		configContainer.RemoveAll()
		configContainer.Refresh()
	}

	// 设置按钮点击事件
	m_btn_local.OnTapped = func() {
		updateWorkTypeButtons(workTypeLocal)
		updateConfigDisplay()
	}
	m_btn_remote.OnTapped = func() {
		updateWorkTypeButtons(workTypeRemote)
		updateConfigDisplay()
	}

	// 初始显示
	updateConfigDisplay()

	// 创建顶部内容（工作模式选择器、密钥输入、配置区域）
	topContent := container.NewVBox(
		workTypeSelector,
		keyInputSection,
		keyButtons,
		configContainer,
	)

	// 创建底部内容（启动按钮和页脚）
	bottomContent := container.NewVBox(
		startButtonContainer,
		NewFooter(pro.GetVersion()),
	)

	// 使用 Border 布局，让日志区域自适应占用剩余空间
	mainContent := container.NewBorder(
		topContent,    // 顶部
		bottomContent, // 底部
		nil,           // 左侧
		nil,           // 右侧
		NewLogList(),  // 中心（自适应区域）
	)

	m_button_start.Disable()
	// 让 GetStunIpPort 内部的 STUN 日志也输出到运行日志列表（仅 GUI 设置，cmd 不调用此处）
	stun2.SetExtraLogSink(func(s string) { UILogPrintF(s) })
	go func() {
		// 等窗口/driver 就绪后再更新 UI，避免启动阶段闪退
		fyne.Do(func() {
			if m_button_start != nil {
				m_button_start.Disable()
			}
		})

		for {
			conn, err := net.ListenUDP("udp4", nil)
			if err != nil {
				UILogPrintF("NAT检测: UDP监听失败: " + err.Error())
				time.Sleep(5 * time.Second)
				continue
			}

			_, wanPort1, wanPort2, _ := stun2.GetStunIpPort(conn)
			conn.Close()

			isNAT4 := wanPort1 != wanPort2
			fyne.Do(func() {
				ShowNATHint(isNAT4)
				if m_button_start != nil {
					m_button_start.Enable()
				}
			})
			return
		}
	}()

	// 添加最小外层padding，确保整体有合适的边距
	return container.NewPadded(mainContent)
}
