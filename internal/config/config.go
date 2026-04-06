package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pikvm/kvmd-cloud/internal/config/vars"
)

type ConfigFile struct {
	Path      string
	MustExist bool
}

const ExtractConfigNode = ""

var (
	ConfigFiles  = []ConfigFile{}
	AuthFilepath = ""
	EnvIsHere    = false
)

func init() {
	if s, err := os.Stat(".env"); err == nil && s.IsDir() {
		EnvIsHere = true
	}
	if _, err := os.Stat(".env/main.yaml"); vars.Debug && err == nil {
		AuthFilepath = ".env/auth.yaml"
		ConfigFiles = append(ConfigFiles, ConfigFile{Path: ".env/main.yaml", MustExist: false})
	} else {
		AuthFilepath = "/etc/kvmd/cloud/auth.yaml"
		ConfigFiles = append(ConfigFiles, ConfigFile{Path: "/etc/kvmd/cloud/main.yaml", MustExist: true})
		ConfigFiles = append(ConfigFiles, ConfigFile{Path: "/etc/kvmd/cloud/override.yaml", MustExist: false})
	}
	ConfigFiles = append(ConfigFiles, ConfigFile{Path: AuthFilepath, MustExist: false})
}

type Config struct {
	AuthToken     string            `json:"auth_token" mapstructure:"auth_token"`
	NoSSL         bool              `json:"nossl" mapstructure:"nossl"`
	SSL           SSLConfigSection  `json:"ssl" mapstructure:"ssl"`
	Hive          HiveConfigSection `json:"hive" mapstructure:"hive"`
	UnixCtlSocket string            `json:"unix_ctl_socket" mapstructure:"unix_ctl_socket"`
	Log           LogConfigSection  `json:"log" mapstructure:"log"`
}

type SSLConfigSection struct {
	Ca string `json:"ca" mapstructure:"ca"`
	// Cert string `json:"cert" mapstructure:"cert"`
	// Key  string `json:"key" mapstructure:"key"`
}

type HiveConfigSection struct {
	Endpoint string `json:"endpoint" mapstructure:"endpoint"`
}

type LogConfigSection struct {
	Level  string    `json:"level" mapstructure:"level"`
	File   string    `json:"file" mapstructure:"file"`
	Tee    bool      `json:"tee" mapstructure:"tee"`
	Format LogFormat `json:"format" mapstructure:"format"`
	Trace  bool      `json:"trace" mapstructure:"trace"`
}

var DefConfig = Config{
	Hive: HiveConfigSection{
		Endpoint: "https://pikvm.cloud",
	},
	UnixCtlSocket: "/run/kvmd/cloud-ctl.sock",
	Log: LogConfigSection{
		Level:  "info",
		File:   "-",
		Format: LogFormatText,
	},
}

var Cfg *Config = nil

func DumpConfig() error {
	s, err := json.MarshalIndent(Cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	fmt.Println(string(s))
	return nil
}
