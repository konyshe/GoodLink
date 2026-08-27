package ui2

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"goodlink/config"
	"goodlink/utils"
)

var (
	upgradeDlMu    sync.Mutex
	upgradeZipPath string
	upgradeDlBusy  bool
)

func cleanupUpgradeResidues() {
	exe := utils.GetCmdExePath()
	_ = os.Remove(exe + ".old")
	_ = os.Remove(filepath.Join(filepath.Dir(exe), "wintun.dll.old"))
}

func HandleUpgradeInfo() (map[string]any, error) {
	info, err := GetUpgradeInfo()
	if err != nil {
		return nil, err
	}
	need := info.Latest != "" && info.Latest != config.GetVersion()
	setUpgradeInfo(need, info.Latest, info.Body, info.DownloadURL)
	return map[string]any{
		"version":     info.Latest,
		"body":        info.Body,
		"downloadUrl": info.DownloadURL,
	}, nil
}

func HandleUpgradeDownload() error {
	info, err := GetUpgradeInfo()
	if err != nil {
		return err
	}
	if info.DownloadURL == "" {
		return fmt.Errorf("未找到当前平台的升级包 (%s)", upgradeZipName())
	}

	upgradeDlMu.Lock()
	if upgradeDlBusy {
		upgradeDlMu.Unlock()
		return fmt.Errorf("正在下载")
	}
	upgradeDlBusy = true
	upgradeDlMu.Unlock()

	go runDownload(info.DownloadURL)
	return nil
}

func runDownload(url string) {
	defer func() {
		upgradeDlMu.Lock()
		upgradeDlBusy = false
		upgradeDlMu.Unlock()
	}()

	setUpgradeProgress(true, 0, false, "")

	dir := filepath.Join(os.TempDir(), "goodlink-update")
	if err := os.MkdirAll(dir, 0755); err != nil {
		setUpgradeProgress(false, 0, false, "创建临时目录失败: "+err.Error())
		return
	}
	dest := filepath.Join(dir, upgradeZipName())

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		setUpgradeProgress(false, 0, false, err.Error())
		return
	}
	req.Header.Set("User-Agent", "Goodlink/"+config.GetVersion())

	resp, err := upgradeHTTPCli.Do(req)
	if err != nil {
		setUpgradeProgress(false, 0, false, "下载失败: "+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		setUpgradeProgress(false, 0, false, fmt.Sprintf("下载失败, HTTP %d", resp.StatusCode))
		return
	}

	tmp := dest + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		setUpgradeProgress(false, 0, false, "保存失败: "+err.Error())
		return
	}

	pw := &progressWriter{w: out, total: resp.ContentLength}
	_, err = io.Copy(pw, resp.Body)
	closeErr := out.Close()
	if err != nil {
		_ = os.Remove(tmp)
		setUpgradeProgress(false, pw.lastP, false, "下载失败: "+err.Error())
		return
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		setUpgradeProgress(false, pw.lastP, false, "保存失败: "+closeErr.Error())
		return
	}
	_ = os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		setUpgradeProgress(false, pw.lastP, false, "保存失败: "+err.Error())
		return
	}

	upgradeDlMu.Lock()
	upgradeZipPath = dest
	upgradeDlMu.Unlock()
	setUpgradeProgress(false, 100, true, "")
}

type progressWriter struct {
	w     io.Writer
	total int64
	read  int64
	lastP int
	lastT time.Time
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.read += int64(n)
	pct := 0
	if p.total > 0 {
		pct = int(p.read * 100 / p.total)
		if pct > 100 {
			pct = 100
		}
	}
	now := time.Now()
	if pct != p.lastP && (pct >= p.lastP+1 || pct == 100 || now.Sub(p.lastT) >= 200*time.Millisecond) {
		p.lastP = pct
		p.lastT = now
		setUpgradeProgress(true, pct, false, "")
	}
	return n, err
}

func HandleUpgradeApply() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("自动更新仅支持 Windows")
	}
	st := snapshotUpgrade()
	upgradeDlMu.Lock()
	zipPath := upgradeZipPath
	busy := upgradeDlBusy
	upgradeDlMu.Unlock()
	if busy {
		return fmt.Errorf("正在下载")
	}
	if !st.Done || zipPath == "" {
		return fmt.Errorf("请先下载完成")
	}
	if _, err := os.Stat(zipPath); err != nil {
		return fmt.Errorf("升级包不存在, 请重新下载")
	}

	StopCmdProcess()

	exe := utils.GetCmdExePath()
	cmd := exec.Command(exe,
		"--self-update",
		"--zip", zipPath,
		"--wait-pid", strconv.Itoa(os.Getpid()),
		"--target", exe,
	)
	hideChildWindow(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动更新进程失败: %v", err)
	}
	_ = cmd.Process.Release()

	go func() {
		time.Sleep(400 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

// RunSelfUpdate 由 --self-update 进程调用: 等待主进程退出, 解压覆盖, 重启主程序.
func RunSelfUpdate(zipPath string, waitPid int, target string) {
	log.Println("self-update start, zip=", zipPath, "waitPid=", waitPid, "target=", target)
	if waitPid > 0 {
		waitForPidExit(waitPid, 45*time.Second)
	}
	time.Sleep(400 * time.Millisecond)

	if target == "" {
		target = utils.GetCmdExePath()
	}
	if zipPath == "" {
		log.Println("self-update: 缺少 zip 路径")
		os.Exit(1)
	}

	if err := applyZip(zipPath, target); err != nil {
		log.Println("self-update failed:", err)
		os.Exit(1)
	}

	cmd := exec.Command(target)
	cmd.Dir = filepath.Dir(target)
	hideChildWindow(cmd)
	if err := cmd.Start(); err != nil {
		log.Println("self-update restart failed:", err)
		os.Exit(1)
	}
	_ = cmd.Process.Release()
	_ = os.Remove(zipPath)
	log.Println("self-update done")
	os.Exit(0)
}

func applyZip(zipPath, target string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开压缩包失败: %w", err)
	}
	defer r.Close()

	var exeSrc, dllSrc *zip.File
	for i := range r.File {
		name := strings.ToLower(filepath.Base(r.File[i].Name))
		switch {
		case strings.HasSuffix(name, ".exe"):
			exeSrc = r.File[i]
		case name == "wintun.dll":
			dllSrc = r.File[i]
		}
	}
	if exeSrc == nil {
		return fmt.Errorf("压缩包中未找到可执行文件")
	}
	if err := extractReplace(exeSrc, target); err != nil {
		return fmt.Errorf("覆盖程序失败: %w", err)
	}
	if dllSrc != nil {
		dllDest := filepath.Join(filepath.Dir(target), "wintun.dll")
		if err := extractReplace(dllSrc, dllDest); err != nil {
			return fmt.Errorf("覆盖 wintun.dll 失败: %w", err)
		}
	}
	return nil
}

func extractReplace(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	tmp := dest + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, rc)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}

	old := dest + ".old"
	_ = os.Remove(old)
	if _, err := os.Stat(dest); err == nil {
		if err := retryRename(dest, old); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	}
	if err := os.Rename(tmp, dest); err != nil {
		if err2 := copyFileOverwrite(tmp, dest); err2 != nil {
			_ = os.Rename(old, dest)
			_ = os.Remove(tmp)
			return err
		}
		_ = os.Remove(tmp)
	}
	return nil
}

func retryRename(oldpath, newpath string) error {
	var err error
	for i := 0; i < 25; i++ {
		err = os.Rename(oldpath, newpath)
		if err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return err
}

func copyFileOverwrite(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}
