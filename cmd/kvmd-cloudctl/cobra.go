package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	ctlclient "github.com/pikvm/kvmd-cloud-agent/internal/ctl/ctlClient"
)

var rootCmd = cobra.Command{
	Use:              "",
	Short:            "A brief description of your application",
	Long:             `A longer description of your application`,
	TraverseChildren: true,
	SilenceUsage:     true,
	SilenceErrors:    true,
}

var statusCmd = cobra.Command{
	Use:   "status",
	Short: "status short",
	Long:  "status long",
	RunE:  ctlclient.RequestStatus,
}

func buildCobra() (*cobra.Command, map[string]*pflag.Flag, error) {
	// 	rootCmd.PersistentFlags().String(
	// 		"unixCtlSocket",
	// 		config.DefaultConfig["unixCtlSocket"].(string),
	// 		"Path to a control unix socket",
	// 	)
	//
	// 	bindToConfig := map[string]*pflag.Flag{
	// 		"unixCtlSocket": rootCmd.PersistentFlags().Lookup("unixCtlSocket"),
	// 	}

	rootCmd.AddCommand(&statusCmd)
	return &rootCmd, nil, nil
}
