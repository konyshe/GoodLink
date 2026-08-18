package config

import (
	"net"
	"strings"
	"testing"
)

func TestNormalizeAndValidate(t *testing.T) {
	cfg := UIConfig{
		LocalMode: "FORWARD",
		ForwardRules: []UIForwardRule{
			{Proto: "TCP", Listen: "127.0.0.1:3389", Remote: "127.0.0.1:3389"},
			{Proto: "udp", Listen: "0.0.0.0:53", Remote: "8.8.8.8:53"},
		},
	}
	if err := NormalizeAndValidate(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.LocalMode != LocalModeForward {
		t.Fatalf("mode=%s", cfg.LocalMode)
	}
	if cfg.ForwardRules[0].Proto != "tcp" || cfg.ForwardRules[0].Listen != "127.0.0.1:3389" {
		t.Fatalf("rule0=%+v", cfg.ForwardRules[0])
	}

	dup := UIConfig{
		LocalMode: "forward",
		ForwardRules: []UIForwardRule{
			{Proto: "tcp", Listen: "127.0.0.1:22", Remote: "127.0.0.1:22"},
			{Proto: "tcp", Listen: "127.0.0.1:22", Remote: "192.168.1.1:22"},
		},
	}
	if err := NormalizeAndValidate(&dup); err == nil {
		t.Fatal("expected duplicate listen error")
	}

	badIP := UIConfig{
		LocalMode: "forward",
		ForwardRules: []UIForwardRule{
			{Proto: "tcp", Listen: "localhost:22", Remote: "127.0.0.1:22"},
		},
	}
	if err := NormalizeAndValidate(&badIP); err == nil {
		t.Fatal("expected ipv4 error")
	}

	empty := UIConfig{}
	if err := NormalizeAndValidate(&empty); err != nil {
		t.Fatal(err)
	}
	if empty.LocalMode != LocalModeTUN {
		t.Fatalf("default mode=%s", empty.LocalMode)
	}
}

func TestListenConflicts(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	listen := ln.Addr().String()
	rule := UIForwardRule{Proto: "tcp", Listen: listen, Remote: "127.0.0.1:1"}

	conflicts := ListenConflicts([]UIForwardRule{rule}, nil)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %v", conflicts)
	}
	if !strings.Contains(conflicts[0], "TCP") || !strings.Contains(conflicts[0], listen) {
		t.Fatalf("conflict text=%q listen=%q", conflicts[0], listen)
	}

	skipped := ListenConflicts([]UIForwardRule{rule}, []UIForwardRule{rule})
	if len(skipped) != 0 {
		t.Fatalf("owned listen should be skipped, got %v", skipped)
	}

	msg := FormatListenConflicts(conflicts)
	if !strings.Contains(msg, "本地端口已被占用") {
		t.Fatalf("msg=%q", msg)
	}
}
