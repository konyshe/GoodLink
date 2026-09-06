package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

//go:embed versioninfo.json
var versionInfoJSON []byte

// 版本号只改 config/versioninfo.json 里的 Version 一处即可。
// Windows 资源中的 FileVersion / ProductVersion 由构建时自动填充。

type versionInfoFile struct {
	Version string `json:"Version"`
}

var version = parseVersion(versionInfoJSON)

func parseVersion(data []byte) string {
	major, minor, patch, _, err := parseVersionParts(data)
	if err != nil {
		panic("config: " + err.Error())
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}

func parseVersionParts(data []byte) (major, minor, patch, build int, err error) {
	var info versionInfoFile
	if err = json.Unmarshal(data, &info); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("解析 versioninfo.json 失败: %w", err)
	}
	v := strings.TrimSpace(info.Version)
	if v == "" {
		return 0, 0, 0, 0, fmt.Errorf("versioninfo.json 缺少 Version")
	}
	parts := strings.Split(v, ".")
	if len(parts) < 3 {
		return 0, 0, 0, 0, fmt.Errorf("Version %q 至少需要 Major.Minor.Patch", v)
	}
	if len(parts) > 4 {
		parts = parts[:4]
	}
	nums := make([]int, 4)
	for i, p := range parts {
		n, convErr := strconv.Atoi(p)
		if convErr != nil {
			return 0, 0, 0, 0, fmt.Errorf("Version %q 无效", v)
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], nums[3], nil
}

func GetVersion() string {
	return version
}

// WriteExpandedVersionInfo 把 Version 展开成 goversioninfo 所需的完整 JSON。
func WriteExpandedVersionInfo(w io.Writer) error {
	out, err := expandedVersionInfoJSON(versionInfoJSON)
	if err != nil {
		return err
	}
	_, err = w.Write(out)
	return err
}

func expandedVersionInfoJSON(data []byte) ([]byte, error) {
	major, minor, patch, build, err := parseVersionParts(data)
	if err != nil {
		return nil, err
	}

	var info map[string]any
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	delete(info, "Version")

	fv := map[string]any{
		"Major": major,
		"Minor": minor,
		"Patch": patch,
		"Build": build,
	}
	ffi, ok := info["FixedFileInfo"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("versioninfo.json 缺少 FixedFileInfo")
	}
	ffi["FileVersion"] = fv
	ffi["ProductVersion"] = fv

	ver := fmt.Sprintf("%d.%d.%d.%d", major, minor, patch, build)
	sfi, ok := info["StringFileInfo"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("versioninfo.json 缺少 StringFileInfo")
	}
	sfi["FileVersion"] = ver
	sfi["ProductVersion"] = ver

	out, err := json.MarshalIndent(info, "", "\t")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
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
