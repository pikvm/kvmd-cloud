package config

import (
	"io"
	"os"

	"github.com/knadh/koanf/maps"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/cliflagv3"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"github.com/pikvm/kvmd-cloud/internal/config/vars"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

const logTimeFormat = "2006-01-02T15:04:05.000000Z07:00"

func subtreeMerger(subtree string, strict bool) func(src, dest map[string]any) error {
	return func(src, dest map[string]any) error {
		if h, ok := src[subtree]; ok {
			if hMap, ok := h.(map[string]any); ok {
				if strict {
					maps.MergeStrict(hMap, dest)
				} else {
					maps.Merge(hMap, dest)
				}
			}
		}
		return nil
	}
}

// LoadConfig loads configuration from files and CLI flags, and sets up the logger.
// It will terminate the program on errors.
func LoadConfig(cmd *cli.Command) {
	strict := false
	k := koanf.NewWithConf(koanf.Conf{
		Delim:       ".",
		StrictMerge: strict,
	})
	if err := k.Load(structs.Provider(DefConfig, "json"), nil); err != nil {
		log.Fatal().Err(err).Msg("Unable to load default config")
		return
	}

	var cfg Config

	if len(cmd.StringSlice("config")) > 0 {
		ConfigFiles = make([]ConfigFile, 0, len(cmd.StringSlice("config")))
		for _, cfgFile := range cmd.StringSlice("config") {
			ConfigFiles = append(ConfigFiles, ConfigFile{Path: cfgFile, MustExist: true})
		}
	}

	mergerOpts := []koanf.Option{}
	if ExtractConfigNode != "" {
		mergerOpts = append(mergerOpts, koanf.WithMergeFunc(subtreeMerger(ExtractConfigNode, strict)))
	}
	for _, configFile := range ConfigFiles {
		if stat, err := os.Stat(configFile.Path); err == nil && stat.Mode().IsRegular() {
			log.Debug().Str("file", configFile.Path).Msg("Loading config file")
			if err := k.Load(file.Provider(configFile.Path), yaml.Parser(), mergerOpts...); err != nil {
				log.Fatal().Err(err).Str("file", configFile.Path).Msg("Unable to load config file")
				return
			}
			log.Debug().Str("file", configFile.Path).Msg("Loaded config file")
		} else if configFile.MustExist {
			log.Fatal().Err(err).Str("file", configFile.Path).Msg("Config file does not exist or is not a regular file")
			return
		}
	}

	const FLAGS_DELIM = "-"
	mergerOpts = []koanf.Option{koanf.WithMergeFunc(flagsMerger(cmd, FLAGS_DELIM, strict))}
	if err := k.Load(cliflagv3.Provider(cmd, FLAGS_DELIM), nil, mergerOpts...); err != nil {
		log.Fatal().Err(err).Msg("Unable to load CLI flags")
		return
	}

	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "json"}); err != nil {
		log.Fatal().Err(err).Msg("Unable to unmarshal default config")
		return
	}

	if err := configPostProcess(&cfg); err != nil {
		log.Fatal().Err(err).Msg("Config post-processing failed")
		return
	}
	Cfg = &cfg
	// DumpConfig()

	setupLogger()
}

func setupLogger() {
	level, err := zerolog.ParseLevel(Cfg.Log.Level)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid log level")
		return
	}
	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = logTimeFormat

	var writer io.Writer
	if Cfg.Log.File == "-" {
		writer = os.Stderr
	} else {
		f, err := os.OpenFile(Cfg.Log.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal().Err(err).Msg("Unable to open log file")
			return
		}
		if Cfg.Log.Tee {
			writer = zerolog.MultiLevelWriter(os.Stderr, f)
		} else {
			writer = f
		}
	}
	if Cfg.Log.Format == LogFormatText {
		writer = zerolog.ConsoleWriter{Out: writer, TimeFormat: zerolog.TimeFieldFormat}
	}

	newLoggerContext := zerolog.New(writer).With().Timestamp()
	if Cfg.Log.Trace {
		newLoggerContext = newLoggerContext.Caller()
	}
	log.Logger = newLoggerContext.Logger()
	zerolog.DefaultContextLogger = &log.Logger
}

func InitBootstrapLogger() {
	zerolog.TimeFieldFormat = logTimeFormat
	zerolog.SetGlobalLevel(map[bool]zerolog.Level{true: zerolog.DebugLevel, false: zerolog.InfoLevel}[vars.Debug])
	log.Logger = zerolog.New(os.Stderr).With().Timestamp().Caller().Logger()
	zerolog.DefaultContextLogger = &log.Logger
}
