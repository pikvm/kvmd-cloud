package config

import (
	"encoding/json"
	"fmt"

	"github.com/xornet-sl/xcommon"

	"github.com/pikvm/kvmd-cloud/internal/config/vars"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type Config struct {
	AuthToken string `json:"auth_token" mapstructure:"auth_token"`
	NoSSL     bool   `json:"nossl" mapstructure:"nossl"`
	SSL       struct {
		Ca string `json:"ca" mapstructure:"ca"`
		// Cert string `json:"cert" mapstructure:"cert"`
		// Key  string `json:"key" mapstructure:"key"`
	} `json:"ssl" mapstructure:"ssl"`
	Hive struct {
		Endpoints []string `json:"endpoints" mapstructure:"endpoints"`
	} `json:"hive" mapstructure:"hive"`
	UnixCtlSocket string `json:"unix_ctl_socket" mapstructure:"unix_ctl_socket"`
	AgentName     string `json:"agent_name" mapstructure:"agent_name"`
	Log           struct {
		Level string `json:"level" mapstructure:"level"`
		File  string `json:"file" mapstructure:"file"`
		Trace bool   `json:"trace" mapstructure:"trace"`
	} `json:"log" mapstructure:"log"`
}

var DefaultConfig = map[string]interface{}{
	"unix_ctl_socket": "/run/kvmd/cloud2.sock",
	"hive.endpoints":  []string{"pikvm.cloud:9000"},
	"log.level":       "info",
	"log.file":        "-",
	"log.trace":       false,
}

var Cfg *Config = nil
var ConfigurationResult *xcommon.ConfigurationResult = nil

var ConfigPlan = xcommon.ConfigurePlan{
	ConfigParsingRules: xcommon.ViperConfig{
		SearchDirs:  []string{vars.BaseConfigDir},
		SearchFiles: []string{vars.MainConfigName, vars.AuthConfigName},
	},
}

func Initialize(cobraBuilder xcommon.CobraBuilderFunc) (*cobra.Command, error) {
	// Enable early logging
	lvl := "info"
	if vars.Debug {
		lvl = "debug"
	}
	if err := xcommon.SetInitialLogger(lvl, "-", false); err != nil {
		log.WithError(err).Warn("Unable to set up initial logger properly")
	}

	rootCmd, bindFlags, err := cobraBuilder()
	if err != nil {
		return nil, err
	}
	if bindFlags == nil {
		bindFlags = make(map[string]*pflag.Flag)
	}
	amendWithCommonFlags(rootCmd, bindFlags)

	return xcommon.InitCobra(
		func() (*cobra.Command, map[string]*pflag.Flag, error) {
			return rootCmd, bindFlags, nil
		},
		&ConfigPlan,
		DefaultConfig,
		&Cfg,
		&ConfigurationResult,
		configureLogger,
	)
}

func amendWithCommonFlags(rootCmd *cobra.Command, bindFlags map[string]*pflag.Flag) {
	flagset := getGlobalFlags()
	rootCmd.PersistentFlags().AddFlagSet(flagset)
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if flagset.Lookup(f.Name) != nil {
			bindFlags[f.Name] = f
		}
	})
}

func configureLogger() {
	if err := xcommon.SetInitialLogger(Cfg.Log.Level, Cfg.Log.File, Cfg.Log.Trace); err != nil {
		log.WithError(err).Error("Unable to set up logger properly")
	}
	if len(ConfigurationResult.ParsedConfigs) > 0 {
		log.Debugf("Configurations loaded successfully: %v", ConfigurationResult.ParsedConfigs)
	} else {
		log.Warn("No configuration found")
	}
}

func DumpConfig() error {
	s, _ := json.MarshalIndent(Cfg, "", "  ")
	fmt.Println(string(s))
	return nil
}
