package ui2

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"goodlink/config"
)

//go:embed web
var webFS embed.FS

const defaultUIAddr = "0.0.0.0:16780"

func StartServer() (string, error) {
	ln, err := net.Listen("tcp4", defaultUIAddr)
	if err != nil {
		// 固定端口占用时回退到随机端口
		ln, err = net.Listen("tcp4", "0.0.0.0:0")
		if err != nil {
			return "", err
		}
	}
	log.Println("UI server started on", ln.Addr().String())

	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		ln.Close()
		return "", err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", handleState)
	mux.HandleFunc("/api/start", handleStart)
	mux.HandleFunc("/api/stop", handleStop)
	mux.HandleFunc("/api/forwards", handleForwards)
	mux.HandleFunc("/api/key/generate", handleGenerateKey)
	mux.HandleFunc("/api/events", handleEvents)
	mux.Handle("/", http.FileServer(http.FS(webContent)))

	go func() {
		_ = http.Serve(ln, mux)
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

func handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		WorkType     string                 `json:"workType"`
		TunKey       string                 `json:"tunKey"`
		LocalMode    string                 `json:"localMode"`
		ForwardRules []config.UIForwardRule `json:"forwardRules"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := HandleStart(req.WorkType, req.TunKey, req.LocalMode, req.ForwardRules); err != nil {
		code := http.StatusBadRequest
		if err.Error() == "already started" || err.Error() == "busy" {
			code = http.StatusConflict
		}
		writeJSON(w, code, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snapshot(false))
}

func handleForwards(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		LocalMode    string                 `json:"localMode"`
		ForwardRules []config.UIForwardRule `json:"forwardRules"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if err := HandleForwards(req.LocalMode, req.ForwardRules); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
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
