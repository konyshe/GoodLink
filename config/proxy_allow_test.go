package config

import (
	"encoding/json"
	"testing"
)

func TestLocalProxyAllowed(t *testing.T) {
	var svc UIService
	if !svc.LocalProxyAllowed() {
		t.Fatal("缺省应允许")
	}

	deny := false
	svc.AllowLocalProxy = &deny
	if svc.LocalProxyAllowed() {
		t.Fatal("显式 false 应拒绝")
	}

	allow := true
	svc.AllowLocalProxy = &allow
	if !svc.LocalProxyAllowed() {
		t.Fatal("显式 true 应允许")
	}
}

func TestAllowLocalProxyJSONOmitted(t *testing.T) {
	var svc UIService
	if err := json.Unmarshal([]byte(`{"name":"a"}`), &svc); err != nil {
		t.Fatal(err)
	}
	if !svc.LocalProxyAllowed() {
		t.Fatal("旧配置缺字段应允许")
	}
	if err := json.Unmarshal([]byte(`{"allow_local_proxy":false}`), &svc); err != nil {
		t.Fatal(err)
	}
	if svc.LocalProxyAllowed() {
		t.Fatal("allow_local_proxy=false 应拒绝")
	}
}
