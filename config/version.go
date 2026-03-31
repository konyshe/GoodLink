package config

import "strings"

var (
	m_version string
)

func SetVersion(v string) {
	m_version = v
}

func GetVersion() string {
	return m_version
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
