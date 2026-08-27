package ui2

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	upgradeInfoMu  sync.Mutex
	cachedUpgrade  upgradeReleaseInfo
	upgradeHTTPCli = &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	upgradeHTTPFast = &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
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

// GetUpgradeInfo 返回缓存的更新说明和下载地址；若尚未拉取则即时请求。
func GetUpgradeInfo() (upgradeReleaseInfo, error) {
	cached := getCachedUpgradeInfo()
	if cached.Latest != "" && cached.DownloadURL != "" {
		return cached, nil
	}
	rel, err := fetchLatestRelease()
	if err != nil {
		if cached.Latest != "" {
			return cached, nil
		}
		return upgradeReleaseInfo{}, err
	}
	return storeUpgradeCache(rel), nil
}

// checkLatestVersion 请求 Gitee API 获取最新版本号，与当前版本比较。
// 返回 (需要升级, 最新版本号)。
func checkLatestVersion(currentVersion string) (bool, string) {
	rel, err := fetchLatestRelease()
	if err != nil {
		log.Println("检查版本失败:", err)
		return false, ""
	}

	info := storeUpgradeCache(rel)
	if info.Latest != "" && info.Latest != currentVersion {
		return true, info.Latest
	}
	return false, info.Latest
}

func startUpgradeCheck() {
	go func() {
		needUpgrade, latestVer := checkLatestVersion(config.GetVersion())
		if needUpgrade {
			info := getCachedUpgradeInfo()
			setUpgradeInfo(true, latestVer, info.Body, info.DownloadURL)
		}
	}()
}
