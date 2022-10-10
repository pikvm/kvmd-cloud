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
	UnixCtlSocket string `json:"unixCtlSocket"`
	AgentName     string `json:"agentName"`
	Log           struct {
		Level string `json:"level"`
		File  string `json:"file"`
		Trace bool   `json:"trace"`
	} `json:"log"`
	// TODO: tmp settings
	ProxyAddress string `json:"proxyAddress"`
}

var DefaultConfig = map[string]interface{}{
	"unixCtlSocket": "/run/kvmd/cloud.sock",
	"log.level":     "info",
	"log.file":      "-",
	"log.trace":     false,
}

var Cfg *Config = nil
var ConfigurationResult *xcommon.ConfigurationResult = nil

var ConfigPlan = xcommon.ConfigurePlan{
	ConfigParsingRules: xcommon.ViperConfig{
		SearchDirs:     []string{vars.BaseConfigDir},
		SearchFiles:    []string{vars.MainConfigName, vars.OverrideConfigName},
		ExtractSubtree: vars.ExtractConfigNode,
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
		log.Info("No configuration found")
	}
}

func DumpConfig() error {
	s, _ := json.MarshalIndent(Cfg, "", "  ")
	fmt.Println(string(s))
	return nil
}
