package config

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestParseVersion(t *testing.T) {
	got := parseVersion([]byte(`{"Version": "1.2.3"}`))
	if got != "1.2.3" {
		t.Fatalf("got %q, want 1.2.3", got)
	}
}

func TestParseVersionDropsBuild(t *testing.T) {
	got := parseVersion([]byte(`{"Version": "1.2.3.4"}`))
	if got != "1.2.3" {
		t.Fatalf("got %q, want 1.2.3", got)
	}
}

func TestGetVersionMatchesJSON(t *testing.T) {
	major, minor, patch, _, err := parseVersionParts(versionInfoJSON)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("%d.%d.%d", major, minor, patch)
	if GetVersion() != want {
		t.Fatalf("GetVersion()=%q, json Version 展开为 %q", GetVersion(), want)
	}
}

func TestExpandedVersionInfoJSON(t *testing.T) {
	out, err := expandedVersionInfoJSON([]byte(`{
		"Version": "9.8.7",
		"FixedFileInfo": {"FileFlagsMask": "3f"},
		"StringFileInfo": {"ProductName": "Goodlink"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	var info struct {
		Version       string `json:"Version"`
		FixedFileInfo struct {
			FileVersion    fileVersion `json:"FileVersion"`
			ProductVersion fileVersion `json:"ProductVersion"`
		} `json:"FixedFileInfo"`
		StringFileInfo struct {
			FileVersion    string `json:"FileVersion"`
			ProductVersion string `json:"ProductVersion"`
		} `json:"StringFileInfo"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		t.Fatal(err)
	}
	if info.Version != "" {
		t.Fatalf("展开后应去掉 Version 字段, got %q", info.Version)
	}
	want := fileVersion{Major: 9, Minor: 8, Patch: 7, Build: 0}
	if info.FixedFileInfo.FileVersion != want || info.FixedFileInfo.ProductVersion != want {
		t.Fatalf("numeric version: file=%v product=%v want=%v",
			info.FixedFileInfo.FileVersion, info.FixedFileInfo.ProductVersion, want)
	}
	if info.StringFileInfo.FileVersion != "9.8.7.0" || info.StringFileInfo.ProductVersion != "9.8.7.0" {
		t.Fatalf("string version: file=%q product=%q",
			info.StringFileInfo.FileVersion, info.StringFileInfo.ProductVersion)
	}
}

type fileVersion struct {
	Major int `json:"Major"`
	Minor int `json:"Minor"`
	Patch int `json:"Patch"`
	Build int `json:"Build"`
}
