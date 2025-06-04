//go:build envishere

package main

import (
	"github.com/pikvm/kvmd-cloud/internal/config"
)

func init() {
	config.ConfigPlan.ConfigParsingRules.ConcreeteFilePaths = []string{".env/main.yaml"}
	authFilepath = ".env/auth.yaml"
	nginxFilepath = ".env/nginx.ctx-http.conf"
	envishere = true
}
