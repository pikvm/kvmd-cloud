package config

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/maps"
	"github.com/urfave/cli/v3"
)

func envVarsSrc(keys ...string) cli.ValueSourceChain {
	srcs := make([]cli.ValueSource, 0, len(keys))
	for _, key := range keys {
		srcs = append(srcs, cli.EnvVar("KVMD_CLOUD_"+key))
	}
	return cli.NewValueSourceChain(srcs...)
}

func GetGlobalFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:  "run",
			Usage: "Run kvmd-cloud. Without this flag kvmd-cloud won't start",
			Value: false,
		},
		&cli.StringFlag{
			Name:    "unix-ctl-socket",
			Usage:   "local ctl socket file",
			Value:   DefConfig.UnixCtlSocket,
			Sources: envVarsSrc("UNIX_CTL_SOCKET"),
		},
		&cli.StringFlag{
			Name:     "log-level",
			Usage:    "Log level (debug|info|warn|error|fatal|panic)",
			Category: "Logging",
			Value:    DefConfig.Log.Level,
			Sources:  envVarsSrc("LOG_LEVEL"),
		},
		&cli.StringFlag{
			Name:     "log-file",
			Usage:    "Log file. use '-' for stderr",
			Category: "Logging",
			Value:    DefConfig.Log.File,
			Sources:  envVarsSrc("LOG_FILE"),
		},
		&cli.BoolFlag{
			Name:     "log-trace",
			Usage:    "Trace output with file and line number. For development use only",
			Category: "Logging",
			Value:    DefConfig.Log.Trace,
			Sources:  envVarsSrc("LOG_TRACE"),
			Hidden:   true,
		},
		&cli.BoolFlag{
			Name:     "log-tee",
			Usage:    "Tee log output to both file and stderr",
			Category: "Logging",
			Value:    DefConfig.Log.Tee,
			Sources:  envVarsSrc("LOG_TEE"),
			Hidden:   false,
		},
		&cli.StringSliceFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Path to configuration file. Can be specified multiple times to load multiple files. Loaded and merged in order",
		},
		&cli.BoolFlag{
			Name:    "version",
			Aliases: []string{"v"},
			Usage:   "display version and exit",
			Local:   true,
		},
	}
}

func flagsMerger(delim string, strict bool) func(src, dest map[string]any) error {
	return func(src, dest map[string]any) error {
		// cliflagv3.Provider produces a map tree with top-level single key.
		// We need to remove that top-level key and merge the subtree with the dest map.
		flat, paths := maps.Flatten(src, nil, delim)
		destFlat, _ := maps.Flatten(dest, nil, delim)
		newFlat := make(map[string]any)
		newPaths := make(map[string][]string)
		rootKeys := make(map[string]struct{})
		for path, value := range flat {
			rootKeys[paths[path][0]] = struct{}{}
			newPath := strings.TrimPrefix(path, paths[path][0]+delim)

			// Filter out flags that are not part of the config
			if _, ok := destFlat[newPath]; !ok {
				continue
			}

			newFlat[newPath] = value
			newPaths[newPath] = paths[path][1:]
		}
		if len(rootKeys) > 1 {
			return fmt.Errorf("Unexpected number of root CLI commands %d > 1", len(rootKeys))
		}
		unflat := maps.Unflatten(newFlat, delim)
		if strict {
			return maps.MergeStrict(unflat, dest)
		}
		maps.Merge(unflat, dest)
		return nil
	}
}
