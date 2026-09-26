package ui2

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"goodlink/config"
)

type giteeRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type upgradeReleaseInfo struct {
	Latest      string
	Body        string
	DownloadURL string
}

var (
	upgradeInfoMu sync.Mutex
	cachedUpgrade upgradeReleaseInfo

	// 升级链路必须验证服务端证书: 跳过校验后, 中间人可以把任意 zip 换成恶意包,
	// 而唯一的完整性校验与被校验文件在同一个 zip 里(自包含), 无法防御这种替换。
	//
	// 下载用的客户端不设整体 Timeout: 升级包较大, 整体超时会中断正常下载。
	// 改用拨号/首字节/握手超时 + 空闲超时, 避免连接永久挂起。
	upgradeHTTPCli = &http.Client{
		Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       60 * time.Second,
		},
	}
	upgradeHTTPFast = &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
)

func upgradeZipName() string {
	return fmt.Sprintf("goodlink-%s-%s.zip", runtime.GOOS, runtime.GOARCH)
}

func findAssetURL(rel giteeRelease, zipName string) string {
	for _, a := range rel.Assets {
		if strings.EqualFold(a.Name, zipName) && a.BrowserDownloadURL != "" {
			return a.BrowserDownloadURL
		}
	}
	tag := rel.TagName
	if tag == "" {
		return ""
	}
	return "https://gitee.com/konyshe/goodlink/releases/download/" + tag + "/" + zipName
}

func fetchLatestRelease() (giteeRelease, error) {
	resp, err := upgradeHTTPFast.Get("https://gitee.com/api/v5/repos/konyshe/goodlink/releases/latest")
	if err != nil {
		return giteeRelease{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return giteeRelease{}, fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return giteeRelease{}, err
	}

	var release giteeRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return giteeRelease{}, err
	}
	return release, nil
}

func storeUpgradeCache(rel giteeRelease) upgradeReleaseInfo {
	info := upgradeReleaseInfo{
		Latest:      strings.TrimPrefix(rel.TagName, "v"),
		Body:        strings.TrimSpace(rel.Body),
		DownloadURL: findAssetURL(rel, upgradeZipName()),
	}
	upgradeInfoMu.Lock()
	cachedUpgrade = info
	upgradeInfoMu.Unlock()
	return info
}

func getCachedUpgradeInfo() upgradeReleaseInfo {
	upgradeInfoMu.Lock()
	defer upgradeInfoMu.Unlock()
	return cachedUpgrade
}

// GetUpgradeInfo 实时请求最新的更新说明和下载地址；网络失败时回退到缓存。
func GetUpgradeInfo() (upgradeReleaseInfo, error) {
	rel, err := fetchLatestRelease()
	if err != nil {
		if cached := getCachedUpgradeInfo(); cached.Latest != "" {
			return cached, nil
		}
		return upgradeReleaseInfo{}, err
	}
	return storeUpgradeCache(rel), nil
}

// checkLatestVersion 请求 Gitee API 获取最新版本号，与当前版本比较。
func checkLatestVersion(currentVersion string) (bool, string, error) {
	rel, err := fetchLatestRelease()
	if err != nil {
		log.Println("检查版本失败:", err)
		return false, "", err
	}

	info := storeUpgradeCache(rel)
	if info.Latest != "" && info.Latest != currentVersion {
		return true, info.Latest, nil
	}
	return false, info.Latest, nil
}

var (
	upgradeCheckMu   sync.Mutex
	upgradeChecking  bool
	upgradeCheckedAt time.Time
)

// runUpgradeCheck 拉取最新版本并更新升级状态。minInterval 内的重复请求会被跳过，
// 避免浏览器多标签或频繁刷新时对 Gitee 造成并发请求。
func runUpgradeCheck(minInterval time.Duration) {
	upgradeCheckMu.Lock()
	if upgradeChecking {
		upgradeCheckMu.Unlock()
		return
	}
	if minInterval > 0 && !upgradeCheckedAt.IsZero() && time.Since(upgradeCheckedAt) < minInterval {
		upgradeCheckMu.Unlock()
		return
	}
	upgradeChecking = true
	upgradeCheckMu.Unlock()

	go func() {
		defer func() {
			upgradeCheckMu.Lock()
			upgradeChecking = false
			upgradeCheckedAt = time.Now()
			upgradeCheckMu.Unlock()
		}()

		needUpgrade, latestVer, err := checkLatestVersion(config.GetVersion())
		if err != nil {
			setUpgradeCheckError("检测更新失败")
			return
		}
		info := getCachedUpgradeInfo()
		setUpgradeInfo(needUpgrade, latestVer, info.Body, info.DownloadURL)
	}()
}

func startUpgradeCheck() {
	runUpgradeCheck(0)
}

// TriggerUpgradeCheck 供 UI 连接时调用，每次打开/刷新页面重新检测版本。
func TriggerUpgradeCheck() {
	runUpgradeCheck(15 * time.Minute)
}
