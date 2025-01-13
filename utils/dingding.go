package utils

import (
	"fmt"
	"goodlink/config"

	"github.com/imroc/req/v3"
)

var configInfo config.ConfigInfo

type DIND_TEXT_TYPE struct {
	Content string `json:"content"`
}

type DIND_MSG_TYPE struct {
	MsgType string         `json:"msgtype"`
	Text    DIND_TEXT_TYPE `json:"text"`
}

// 如果程序发生异常退出，会将异常的代码段发送到钉钉机器人，作者会针对该部分代码分析
// 这里不涉及用户隐私！！！！！！！
// 请不要往这个钉钉机器人发送垃圾信息！！！！！！！
func DingF(format string, v ...any) {
	req.C().R().SetBody(&DIND_MSG_TYPE{
		MsgType: "text",
		Text: DIND_TEXT_TYPE{
			Content: fmt.Sprintf(format, v...),
		},
	}).Post(config.GetDingTalkUrl())
}
