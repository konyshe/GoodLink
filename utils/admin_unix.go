//go:build unix

package utils

import "os"

func IsAdmin() bool {
	return os.Geteuid() == 0
}
