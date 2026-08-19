package proxy

import (
	"net"
	"strconv"
	"testing"
)

func TestStartBuiltinProxyPrefer1080(t *testing.T) {
	occupy, err := net.Listen("tcp4", net.JoinHostPort(BuiltinProxyHost, strconv.Itoa(PROXY_PORT)))
	portFree := err != nil
	if occupy != nil {
		defer occupy.Close()
	}

	mgr := NewForwardManager()
	defer mgr.Close()
	addr, fallback, err := mgr.StartBuiltinProxy()
	if err != nil {
		t.Fatal(err)
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	if host != BuiltinProxyHost {
		t.Fatalf("host=%s want %s", host, BuiltinProxyHost)
	}
	if portFree {
		if fallback {
			t.Fatalf("1080 空闲时不应回退, addr=%s", addr)
		}
		if portStr != strconv.Itoa(PROXY_PORT) {
			t.Fatalf("port=%s want %d", portStr, PROXY_PORT)
		}
		return
	}
	if !fallback {
		t.Fatalf("1080 占用时应回退, addr=%s", addr)
	}
	if portStr == strconv.Itoa(PROXY_PORT) {
		t.Fatalf("回退后仍是 1080: %s", addr)
	}
}

func TestStartBuiltinProxySurvivesApply(t *testing.T) {
	mgr := NewForwardManager()
	defer mgr.Close()
	addr, _, err := mgr.StartBuiltinProxy()
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Apply(nil); err != nil {
		t.Fatal(err)
	}
	got, _, err := mgr.StartBuiltinProxy()
	if err != nil {
		t.Fatal(err)
	}
	if got != addr {
		t.Fatalf("Apply 后内置代理地址变化: %s -> %s", addr, got)
	}
}
