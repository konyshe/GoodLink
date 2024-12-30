//go:build windows

package ui2

import (
	"encoding/json"
	"errors"
	"gogo"
	"log"
	"sync"
	"time"

	"goodlink/config"
	"goodlink/pro"
	_ "goodlink/pro"
	"goodlink/tools"

	_ "embed"
	_ "net/http/pprof"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/atotto/clipboard"
)

var (
	m_local_ip            string
	m_remote_addr         string
	key_box               *fyne.Container
	myWindow              fyne.Window
	m_start_mg            sync.WaitGroup
	m_start_lock          sync.Mutex
	start_button          *widget.Button
	radio                 *widget.RadioGroup
	keyValidated          *widget.Entry
	localUI               *LocalUI
	remoteUI              *RemoteUI
	key_create_button     *widget.Button
	key_paste_button      *widget.Button
	start_button_activity *widget.Activity
	log_view              *LogLabel
	start_button_stats    int
)

const (
	M_APP_TITLE = "GoodLink"
)

func LogInit(log_view *LogLabel) {
	gogo.Log().RegistInfo(func(content string) {
		SetLogLabel(content)
		log.Println(content)
	})
	gogo.Log().RegistDebug(func(content string) {
		SetLogLabel(content)
	})
	gogo.Log().RegistError(func(content string) {
		SetLogLabel(content)
		fyne.LogError("error: ", errors.New(content))
	})
}

func start_button_click_1(content string) {
	start_button.Disable()
	radio.Disable()
	keyValidated.Disable()
	localUI.Disable()
	key_create_button.Disable()
	key_paste_button.Disable()
	start_button_activity.Start()
	start_button_activity.Show()
	log_view.SetText(content)
	start_button_stats = 1
}

func start_button_click_0() {
	radio.Enable()
	keyValidated.Enable()
	localUI.Enable()
	key_create_button.Enable()
	key_paste_button.Enable()
	start_button_activity.Stop()
	start_button_activity.Hide()
}

func start_button_click() {
	m_start_lock.Lock()
	defer m_start_lock.Unlock()

	var err error
	var remote_addr string
	var local_addr string

	//先对需要填写的数据进行校验
	switch start_button_stats {
	case 0:
		if len(keyValidated.Text) < 16 {
			SetLogLabel("请输入或点击生成连接密钥!")
			return
		}
		switch radio.Selected {
		case "Local":
			if local_addr, err = localUI.GetLocalAddr(); err != nil {
				SetLogLabel(err.Error())
				return
			}
		case "Remote":
			if remote_addr, err = remoteUI.GetRemoteAddr(); err != nil {
				SetLogLabel(err.Error())
				return
			}
		}

		log.Println(local_addr)

		configByte, _ := json.Marshal(&config.ConfigInfo{
			WorkType:   radio.Selected,
			TunKey:     keyValidated.Text,
			ConnType:   localUI.GetConnType2(),
			LocalIP:    localUI.GetLocalIP(),
			LocalPort:  localUI.GetLocalPort(),
			RemoteType: remoteUI.GetRemoteType(),
			RemoteIP:   remoteUI.GetRemoteIP(),
			RemotePort: remoteUI.GetRemotePort(),
		})
		log.Println(string(configByte))
		gogo.Utils().FileDel("goodlink.json")
		gogo.Utils().FileAppend("goodlink.json", configByte)
	}

	switch start_button_stats {
	case 0:
		start_button_click_1("正在启动...")

		go func() {
			time.Sleep(time.Second * 1)
			start_button.Enable()

			for start_button_stats == 1 {
				if pro.GetLocalStats() == 2 {
					log_view.SetText("连接成功")
					break
				}
				time.Sleep(time.Millisecond * 200)
			}
		}()

		switch radio.Selected {
		case "Local":
			m_start_mg.Add(1)
			go func() {
				defer m_start_mg.Done()
				if err := pro.RunLocal(localUI.GetConnType(), remote_addr, keyValidated.Text); err != nil {
					SetLogLabel(err.Error())
					start_button_stats = 0
					return
				}
			}()
		}

	case 1:
		start_button.Disable()
		log_view.SetText("正在停止...")
		start_button_stats = 0

		switch radio.Selected {
		case "Local":
			go func() {
				pro.StopLocal()
				m_start_mg.Wait()
				start_button_click_0()
			}()
		}
	}
}

func GetMainUI(myWindow *fyne.Window) *fyne.Container {
	var configInfo config.ConfigInfo
	json.Unmarshal(gogo.Utils().FileReadAll("goodlink.json"), &configInfo)
	log.Println(configInfo)

	keyValidated = widget.NewEntry()
	keyValidated.SetPlaceHolder("自定义16-24字节长度")
	if len(configInfo.TunKey) > 0 {
		keyValidated.SetText(configInfo.TunKey)
	}

	key_create_button = widget.NewButton("生成密钥", func() {
		keyValidated.SetText(tools.RandomString(24))
	})
	key_copy_button := widget.NewButton("复制密钥", func() {
		clipboard.WriteAll(keyValidated.Text)
	})
	key_paste_button = widget.NewButton("粘贴密钥", func() {
		if s, err := clipboard.ReadAll(); err == nil {
			keyValidated.SetText(s)
		}
	})

	localUI = NewLocalUI(myWindow, &configInfo)
	localUI_Container := localUI.GetContainer()

	remoteUI = NewRemoteUI(myWindow, &configInfo)
	remoteUI_Container := remoteUI.GetContainer()

	radio = widget.NewRadioGroup([]string{"Remote", "Local"}, nil)
	radio.Horizontal = true
	radio.OnChanged = func(value string) {
		switch value {
		case "Remote":
			localUI_Container.Hide()
			remoteUI_Container.Show()
		case "Local":
			remoteUI_Container.Hide()
			localUI_Container.Show()
		default:
			radio.SetSelected("Local")
		}
	}
	if len(configInfo.WorkType) > 0 {
		radio.SetSelected(configInfo.WorkType)
	} else {
		radio.SetSelected("Local")
	}

	log_view = NewLogLabel("等待启动")
	LogInit(log_view)

	start_button_stats = 0
	start_button_activity = widget.NewActivity()
	start_button = widget.NewButton("点击启动", start_button_click)
	start_button.Importance = widget.HighImportance
	start_button.Resize(fyne.NewSize(100, 40))
	start_button.Disable()
	go func() {
		pro.Init("", "", 0)
		start_button.Enable()
	}()

	return container.New(layout.NewVBoxLayout(),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("工作端侧: "), radio),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("连接密钥: "), keyValidated),
		container.NewGridWithColumns(3, key_create_button, key_copy_button, key_paste_button),
		localUI_Container, remoteUI_Container,
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("当前状态: "), log_view),
		container.NewStack(start_button, start_button_activity))
}
