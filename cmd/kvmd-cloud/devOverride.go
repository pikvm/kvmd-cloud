//go:build envishere

package main

import (
	"github.com/pikvm/kvmd-cloud/internal/config"
	"os"
)

func init() {
	cfgFiles := []string{".env/main.yaml"}
	if _, err := os.Stat(".env/auth.yaml"); err == nil {
		cfgFiles = append(cfgFiles, ".env/auth.yaml")
	}
	config.ConfigPlan.ConfigParsingRules.ConcreeteFilePaths = cfgFiles
}
