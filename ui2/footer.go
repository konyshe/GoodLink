//go:build windows

package ui2

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"goodlink/config"
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

func startUpgradeCheck() {
	go func() {
		needUpgrade, latestVer := checkLatestVersion(config.GetVersion())
		if needUpgrade {
			setUpgradeInfo(true, latestVer)
		}
	}()
}
