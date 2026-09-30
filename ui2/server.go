package ui2

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"goodlink/config"
)

//go:embed web
var webFS embed.FS

const (
	// 控制页面绑定 0.0.0.0, 同网段任何主机都能无鉴权地
	// 读取含连接密钥的配置、启停服务、甚至清空全部服务
	defaultUIAddr     = "0.0.0.0:16780"
	uiAddrRetries     = 10
	uiAddrRetryWaitMS = 200

	// maxAPIBodyBytes 限制请求体大小, 避免超大 body 在反序列化阶段打爆内存
	maxAPIBodyBytes = 1 << 20
)

// uiToken 用于放开监听后的远程访问鉴权; 未设置时只允许本机访问
var uiToken = strings.TrimSpace(os.Getenv("GOODLINK_UI_TOKEN"))

// uiListenAddr 允许用环境变量放开监听(例如需要局域网访问 UI),
// 放开后必须同时设置 GOODLINK_UI_TOKEN, 否则远程请求会被拒绝。
func uiListenAddr() string {
	if v := strings.TrimSpace(os.Getenv("GOODLINK_UI_ADDR")); v != "" {
		log.Println("UI 监听地址已被 GOODLINK_UI_ADDR 覆盖:", v)
		return v
	}
	return defaultUIAddr
}

// authorize 校验控制面请求的来源与身份。
// 该接口能导出连接密钥、启停服务、覆盖配置文件, 不能对任意来源开放。
func authorize(w http.ResponseWriter, r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)

	if ip != nil && ip.IsLoopback() {
		// 即使是本机来源也要校验 Host: 外部页面诱导浏览器访问 127.0.0.1 时
		// RemoteAddr 同样是回环, 只有 Host 头能区分(DNS rebinding)
		if !isLocalHostHeader(r.Host) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return false
		}
		return true
	}

	if uiToken == "" {
		http.Error(w, "forbidden: 允许非本机访问需先设置 GOODLINK_UI_TOKEN", http.StatusForbidden)
		return false
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Goodlink-Token")), []byte(uiToken)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

// isLocalHostHeader 判断请求 Host 是否指向本机
func isLocalHostHeader(host string) bool {
	h := host
	if hh, _, err := net.SplitHostPort(host); err == nil {
		h = hh
	}
	h = strings.ToLower(strings.TrimSpace(h))
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// listenUI 优先使用固定端口。端口被占用时重试若干次, 避免回退随机端口导致旧页面无法重连。
func listenUI() (net.Listener, error) {
	addr := uiListenAddr()
	listenHost := defaultUIAddr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		listenHost = h
	} else {
		addr = defaultUIAddr
	}

	var err error
	for i := 0; i < uiAddrRetries; i++ {
		var ln net.Listener
		ln, err = net.Listen("tcp4", addr)
		if err == nil {
			return ln, nil
		}
		if i+1 < uiAddrRetries {
			time.Sleep(uiAddrRetryWaitMS * time.Millisecond)
		}
	}

	log.Println("UI固定端口不可用, 回退随机端口:", err)
	return net.Listen("tcp4", net.JoinHostPort(listenHost, "0"))
}

func StartServer() (string, error) {
	ln, err := listenUI()
	if err != nil {
		return "", err
	}
	log.Println("UI server started on", ln.Addr().String())

	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		ln.Close()
		return "", err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", handleState)
	mux.HandleFunc("POST /api/services", handleCreateService)
	mux.HandleFunc("DELETE /api/services/{id}", handleDeleteService)
	mux.HandleFunc("POST /api/services/{id}/rename", handleRenameService)
	mux.HandleFunc("POST /api/services/{id}/start", handleStartService)
	mux.HandleFunc("POST /api/services/{id}/stop", handleStopService)
	mux.HandleFunc("POST /api/services/{id}/forwards", handleServiceForwards)
	mux.HandleFunc("POST /api/services/{id}/autostart", handleServiceAutoStart)
	mux.HandleFunc("POST /api/services/{id}/config", handleServiceConfig)
	mux.HandleFunc("/api/key/generate", handleGenerateKey)
	mux.HandleFunc("/api/config", handleConfig)
	mux.HandleFunc("/api/events", handleEvents)
	mux.HandleFunc("/api/upgrade/info", handleUpgradeInfo)
	mux.HandleFunc("/api/upgrade/download", handleUpgradeDownload)
	mux.HandleFunc("/api/upgrade/apply", handleUpgradeApply)
	mux.Handle("/", http.FileServer(http.FS(webContent)))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorize(w, r) {
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxAPIBodyBytes)
		}
		mux.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
		// 不设 WriteTimeout: /api/events 是常驻 SSE, 整体写超时会把长连接切断
	}

	go func() {
		_ = srv.Serve(ln)
	}()

	return "http://127.0.0.1:" + strings.Split(ln.Addr().String(), ":")[1] + "/", nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, snapshot(true))
}

type serviceBody struct {
	Name            string                 `json:"name"`
	WorkType        string                 `json:"workType"`
	TunKey          string                 `json:"tunKey"`
	LocalMode       string                 `json:"localMode"`
	Transport       string                 `json:"transport"`
	AutoStart       bool                   `json:"autoStart"`
	AllowLocalProxy *bool                  `json:"allowLocalProxy"`
	ForwardRules    []config.UIForwardRule `json:"forwardRules"`
}

func serviceID(r *http.Request) string {
	return r.PathValue("id")
}

func writeServiceErr(w http.ResponseWriter, err error) {
	code := http.StatusBadRequest
	msg := err.Error()
	if msg == "already started" || msg == "busy" {
		code = http.StatusConflict
	}
	writeJSON(w, code, map[string]string{"error": msg})
}

func handleCreateService(w http.ResponseWriter, r *http.Request) {
	view, err := HandleCreateService()
	if err != nil {
		writeServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func handleDeleteService(w http.ResponseWriter, r *http.Request) {
	if err := HandleDeleteService(serviceID(r)); err != nil {
		writeServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot(false))
}

func handleRenameService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if err := HandleRename(serviceID(r), req.Name); err != nil {
		writeServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot(false))
}

func handleStartService(w http.ResponseWriter, r *http.Request) {
	var req serviceBody
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := HandleStart(serviceID(r), req.WorkType, req.TunKey, req.LocalMode, req.Transport, req.ForwardRules, req.AllowLocalProxy); err != nil {
		writeServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot(false))
}

func handleStopService(w http.ResponseWriter, r *http.Request) {
	HandleStop(serviceID(r))
	writeJSON(w, http.StatusOK, snapshot(false))
}

func handleServiceForwards(w http.ResponseWriter, r *http.Request) {
	var req serviceBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if err := HandleForwards(serviceID(r), req.LocalMode, req.ForwardRules); err != nil {
		writeServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot(false))
}

func handleServiceAutoStart(w http.ResponseWriter, r *http.Request) {
	var req serviceBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if err := HandleSetAutoStart(serviceID(r), req.AutoStart, req.WorkType, req.TunKey, req.LocalMode, req.Transport, req.ForwardRules, req.AllowLocalProxy); err != nil {
		writeServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot(false))
}

func handleServiceConfig(w http.ResponseWriter, r *http.Request) {
	var req serviceBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if err := HandleUpdateConfig(serviceID(r), req.WorkType, req.TunKey, req.LocalMode, req.Transport, req.ForwardRules, req.AutoStart, req.AllowLocalProxy); err != nil {
		writeServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot(false))
}

func handleGenerateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"tunKey": GenerateKey()})
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		data, err := json.MarshalIndent(currentUIConfig(), "", "  ")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "导出失败"})
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="goodlink.json"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(append(data, '\n'))
	case http.MethodPost:
		var cfg config.UIConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "配置文件格式错误"})
			return
		}
		if err := HandleImportConfig(cfg); err != nil {
			code := http.StatusBadRequest
			if err.Error() == "busy" {
				code = http.StatusConflict
			}
			writeJSON(w, code, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, snapshot(false))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleUpgradeInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	info, err := HandleUpgradeInfo()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func handleUpgradeDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := HandleUpgradeDownload(); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func handleUpgradeApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := HandleUpgradeApply(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := sseHubInst.subscribe()
	defer sseHubInst.unsubscribe(ch)

	if _, err := w.Write(sseMessage("snapshot", snapshot(true))); err != nil {
		return
	}
	flusher.Flush()

	// 每次浏览器打开/刷新 UI 时重新检测版本（带短防抖，避免多标签并发请求）
	TriggerUpgradeCheck()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if _, err := w.Write(msg); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
