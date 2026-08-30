package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed versioninfo.json
var versionInfoJSON []byte

type fileVersion struct {
	Major int `json:"Major"`
	Minor int `json:"Minor"`
	Patch int `json:"Patch"`
}

type versionInfoFile struct {
	FixedFileInfo struct {
		ProductVersion fileVersion `json:"ProductVersion"`
	} `json:"FixedFileInfo"`
}

var version = parseVersion(versionInfoJSON)

func parseVersion(data []byte) string {
	var info versionInfoFile
	if err := json.Unmarshal(data, &info); err != nil {
		panic("config: 解析 versioninfo.json 失败: " + err.Error())
	}
	v := info.FixedFileInfo.ProductVersion
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func GetVersion() string {
	return version
}

// VersionsCompatible reports whether two X.Y.Z style version strings are
// wire-compatible: the first two dot-separated segments must match. If either
// string has fewer than two segments, falls back to exact string equality.
func VersionsCompatible(a, b string) bool {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	if len(pa) >= 2 && len(pb) >= 2 {
		return pa[0] == pb[0] && pa[1] == pb[1]
	}
	return a == b
}
