package config

import (
	"github.com/spf13/pflag"
)

// func getConfigFlags() *pflag.FlagSet {
// 	configFlags := pflag.NewFlagSet("config", pflag.ExitOnError)
// 	configFlags.StringP("config", "c", "", "Configuration file")
// 	configFlags.MarkHidden("config")
// 	return configFlags
// }

func getLogFlags() *pflag.FlagSet {
	logFlags := pflag.NewFlagSet("log", pflag.ExitOnError)
	logFlags.String("log.level", DefaultConfig["log.level"].(string), "Log level (debug|info|warn|error|fatal|panic)")
	logFlags.String("log.file", DefaultConfig["log.file"].(string), "Log file. use '-' for stdout")
	logFlags.Bool("log.trace", DefaultConfig["log.trace"].(bool), "Mega deep tracing output. For development use only")
	logFlags.MarkHidden("log.trace")
	return logFlags
}

func getGlobalFlags() *pflag.FlagSet {
	commonFlags := pflag.NewFlagSet("common", pflag.ExitOnError)
	// commonFlags.AddFlagSet(getConfigFlags())
	commonFlags.AddFlagSet(getLogFlags())
	commonFlags.String("unixCtlSocket", DefaultConfig["unixCtlSocket"].(string), "local ctl socket file")
	return commonFlags
}
