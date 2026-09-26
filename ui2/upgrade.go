package ui2

import (
	"archive/zip"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
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

// upgradeRollbackKeep 是 .old 回滚点的保留时长。
// .old 是更新失败时唯一的回滚点, 不能在每次启动时立刻删除。
const upgradeRollbackKeep = 7 * 24 * time.Hour

var (
	upgradeDlMu    sync.Mutex
	upgradeZipPath string
	upgradeDlBusy  bool
)

func cleanupUpgradeResidues() {
	exe := utils.GetCmdExePath()
	removeOldAfter(exe+".old", upgradeRollbackKeep)
	removeOldAfter(filepath.Join(filepath.Dir(exe), "wintun.dll.old"), upgradeRollbackKeep)
}

// removeOldAfter 只清理超过保留时长的回滚文件:
// 新版启动后立刻删除 .old, 一旦新版崩溃循环就再也回不去了
func removeOldAfter(path string, keep time.Duration) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if time.Since(info.ModTime()) < keep {
		return
	}
	_ = os.Remove(path)
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

	// 必须用随机私有目录: 固定的 /tmp/goodlink-update 可被他人预先创建,
	// 在里面放符号链接就能让下载覆盖到任意可写文件
	dir, err := os.MkdirTemp(os.TempDir(), "goodlink-update-")
	if err != nil {
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

	self := utils.GetCmdExePath()
	if target == "" {
		target = self
	}
	if !sameFilePath(target, self) {
		log.Println("self-update: 目标路径无效")
		os.Exit(1)
	}
	if zipPath == "" {
		log.Println("self-update: 缺少 zip 路径")
		os.Exit(1)
	}

	if err := applyZip(zipPath, target); err != nil {
		log.Println("self-update failed:", err)
		// 替换失败必须从 .old 恢复并重启旧版本:
		// 主进程已经退出, 不恢复的话用户会彻底失去可用的程序
		restoreAndRestart(target)
		os.Exit(1)
	}

	cmd := exec.Command(target, "--no-browser")
	cmd.Dir = filepath.Dir(target)
	if err := cmd.Start(); err != nil {
		log.Println("self-update restart failed:", err)
		restoreAndRestart(target)
		os.Exit(1)
	}
	_ = cmd.Process.Release()
	_ = os.Remove(zipPath)
	log.Println("self-update done")
	os.Exit(0)
}

// restoreAndRestart 用 .old 回滚点恢复旧版本并重新启动
func restoreAndRestart(target string) {
	old := target + ".old"
	if _, err := os.Stat(old); err != nil {
		log.Println("self-update: 没有可用的回滚点:", old)
		return
	}
	if err := retryRename(old, target); err != nil {
		log.Println("self-update: 回滚失败:", err)
		return
	}
	cmd := exec.Command(target, "--no-browser")
	cmd.Dir = filepath.Dir(target)
	if err := cmd.Start(); err != nil {
		log.Println("self-update: 重启旧版本失败:", err)
		return
	}
	_ = cmd.Process.Release()
	log.Println("self-update: 已回滚到旧版本")
}

func sameFilePath(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return false
	}
	aa, bb = filepath.Clean(aa), filepath.Clean(bb)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(aa, bb)
	}
	return aa == bb
}

func zipEntryBase(name string) (string, error) {
	n := strings.ReplaceAll(name, "\\", "/")
	if n == "" || strings.HasPrefix(n, "/") || strings.Contains(n, ":") {
		return "", fmt.Errorf("非法路径: %s", name)
	}
	for _, p := range strings.Split(n, "/") {
		if p == ".." {
			return "", fmt.Errorf("非法路径: %s", name)
		}
	}
	base := filepath.Base(n)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "", fmt.Errorf("非法路径: %s", name)
	}
	return base, nil
}

func parseMd5sumFile(s string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.ToLower(filepath.Base(fields[len(fields)-1]))
		name = strings.TrimPrefix(name, "*")
		out[name] = fields[0]
	}
	return out
}

func applyZip(zipPath, target string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开压缩包失败: %w", err)
	}
	defer r.Close()

	destDir := filepath.Dir(target)
	var exeSrc, dllSrc, shaSrc, md5Src *zip.File
	for i := range r.File {
		if r.File[i].FileInfo().IsDir() {
			continue
		}
		base, err := zipEntryBase(r.File[i].Name)
		if err != nil {
			return err
		}
		switch strings.ToLower(base) {
		case "sha256sum.txt":
			shaSrc = r.File[i]
		case "md5sum.txt":
			md5Src = r.File[i]
		case "wintun.dll":
			dllSrc = r.File[i]
		default:
			if strings.HasSuffix(strings.ToLower(base), ".exe") {
				exeSrc = r.File[i]
			}
		}
	}
	if exeSrc == nil {
		return fmt.Errorf("压缩包中未找到可执行文件")
	}

	// 优先 SHA-256: md5 已不适合作为完整性校验。
	// 注意校验清单与待校验文件在同一个 zip 内, 只能防损坏, 防不住整包替换,
	// 因此传输层必须验证证书(见 footer.go 的 upgradeHTTPCli)。
	algo := ""
	var hashes map[string]string
	switch {
	case shaSrc != nil:
		algo = "sha256"
		hashes, err = readHashFile(shaSrc)
	case md5Src != nil:
		log.Println("升级包未提供 sha256sum.txt, 回退使用 md5 校验")
		algo = "md5"
		hashes, err = readHashFile(md5Src)
	}
	if err != nil {
		return err
	}
	if (shaSrc != nil || md5Src != nil) && len(hashes) == 0 {
		return fmt.Errorf("校验清单为空或格式无效")
	}

	exeBase := strings.ToLower(filepath.Base(exeSrc.Name))

	// 先把所有文件解压并校验到临时位置, 全部通过后再统一替换:
	// 原来的顺序替换会在 wintun.dll 失败时留下"新exe + 旧dll"的不兼容组合
	exeTmp := target + ".new"
	if err := extractTo(exeSrc, exeTmp, algo, hashFor(hashes, exeBase)); err != nil {
		_ = os.Remove(exeTmp)
		return fmt.Errorf("解压程序失败: %w", err)
	}

	dllTmp := ""
	if dllSrc != nil {
		dllTmp = filepath.Join(destDir, "wintun.dll") + ".new"
		if err := extractTo(dllSrc, dllTmp, algo, hashFor(hashes, "wintun.dll")); err != nil {
			_ = os.Remove(exeTmp)
			_ = os.Remove(dllTmp)
			return fmt.Errorf("解压 wintun.dll 失败: %w", err)
		}
	}

	if err := replaceFile(exeTmp, target); err != nil {
		_ = os.Remove(exeTmp)
		_ = os.Remove(dllTmp)
		return fmt.Errorf("覆盖程序失败: %w", err)
	}
	if dllTmp != "" {
		if err := replaceFile(dllTmp, filepath.Join(destDir, "wintun.dll")); err != nil {
			return fmt.Errorf("覆盖 wintun.dll 失败: %w", err)
		}
	}
	return nil
}

// readHashFile 读取压缩包内的校验清单
func readHashFile(f *zip.File) (map[string]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("读取校验清单失败: %w", err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil, fmt.Errorf("读取校验清单失败: %w", err)
	}
	hashes := parseMd5sumFile(string(data))
	return hashes, nil
}

func hashFor(hashes map[string]string, base string) string {
	if hashes == nil {
		return ""
	}
	h, ok := hashes[strings.ToLower(base)]
	if !ok {
		return "-"
	}
	return h
}

// extractTo 把压缩包内的文件解压到 dest 并按指定算法校验, 不做替换
func extractTo(f *zip.File, dest string, algo, want string) error {
	if want == "-" {
		return fmt.Errorf("校验清单缺少 %s", filepath.Base(dest))
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}

	var h hash.Hash
	if algo == "sha256" {
		h = sha256.New()
	} else {
		h = md5.New()
	}

	_, copyErr := io.Copy(out, io.TeeReader(rc, h))
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if want != "" {
		got := hex.EncodeToString(h.Sum(nil))
		if !strings.EqualFold(got, want) {
			return fmt.Errorf("%s 校验失败", filepath.Base(dest))
		}
	}
	return nil
}

// replaceFile 用 src 覆盖 dest, 覆盖前保留 .old 作为回滚点。
// 失败时必须尝试回滚并把回滚结果报出来: 原实现把回滚的 rename 错误丢弃,
// 覆盖失败后程序可能已经损坏且无人恢复。
func replaceFile(src, dest string) error {
	old := dest + ".old"
	_ = os.Remove(old)
	if _, err := os.Stat(dest); err == nil {
		if err := retryRename(dest, old); err != nil {
			return err
		}
	}
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	if err := copyFileOverwrite(src, dest); err != nil {
		// 覆盖失败: 用 .old 恢复, 恢复失败也必须把错误报出去, 不能静默
		if _, statErr := os.Stat(old); statErr == nil {
			if rerr := retryRename(old, dest); rerr != nil {
				return fmt.Errorf("覆盖失败且回滚失败: %v(回滚: %v)", err, rerr)
			}
		}
		return err
	}
	_ = os.Remove(src)
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
