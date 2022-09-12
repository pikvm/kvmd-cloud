package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var rootCmd = cobra.Command{
	Use:           "",
	Short:         "A brief description of your application",
	Long:          `A longer description of your application`,
	RunE:          root,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func buildCobra() (*cobra.Command, map[string]*pflag.Flag, error) {
	return &rootCmd, nil, nil
}
