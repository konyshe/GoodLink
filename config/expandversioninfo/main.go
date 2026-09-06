package main

import (
	"fmt"
	"os"

	"goodlink/config"
)

func main() {
	if err := config.WriteExpandedVersionInfo(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
