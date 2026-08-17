//go:build windows

package ui2

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"syscall"
	"time"
)

//go:embed web
var webFS embed.FS

const defaultUIAddr = "127.0.0.1:16780"

func OpenBrowser(url string) {
	cmd := exec.Command("cmd", "/c", "start", "", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	_ = cmd.Start()
}

func StartServer() (string, error) {
	ln, err := net.Listen("tcp", defaultUIAddr)
	if err != nil {
		// 固定端口占用时回退到随机端口
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return "", err
		}
	}

	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		ln.Close()
		return "", err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", handleState)
	mux.HandleFunc("/api/start", handleStart)
	mux.HandleFunc("/api/stop", handleStop)
	mux.HandleFunc("/api/key/generate", handleGenerateKey)
	mux.HandleFunc("/api/events", handleEvents)
	mux.Handle("/", http.FileServer(http.FS(webContent)))

	go func() {
		_ = http.Serve(ln, mux)
	}()

	return "http://" + ln.Addr().String() + "/", nil
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

func handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		WorkType string `json:"workType"`
		TunKey   string `json:"tunKey"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := HandleStart(req.WorkType, req.TunKey); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snapshot(false))
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	HandleStop()
	writeJSON(w, http.StatusOK, snapshot(false))
}

func handleGenerateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !othersEnabled() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "busy"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"tunKey": GenerateKey()})
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
