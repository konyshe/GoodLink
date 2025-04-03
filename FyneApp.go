package main

import (
	_ "embed"
	"log"

	"github.com/BurntSushi/toml"
)

//go:embed FyneApp.toml
var FyneAppToml string

type FyneAppConfig struct {
	Website string `toml:"Website"`
	Details struct {
		Icon    string `toml:"Icon"`
		Name    string `toml:"Name"`
		ID      string `toml:"ID"`
		Version string `toml:"Version"`
		Build   int    `toml:"Build"`
	} `toml:"Details"`
}

func GetVersion() string {
	var config FyneAppConfig
	if _, err := toml.Decode(FyneAppToml, &config); err != nil {
		log.Fatal(err)
	}

	return config.Details.Version
}
