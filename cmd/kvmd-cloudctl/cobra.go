package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	ctlclient "github.com/pikvm/kvmd-cloud/internal/ctl/ctlClient"
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

var authCmd = cobra.Command{
	Use:   "auth",
	Short: "Authorize agent in cloud",
	RunE:  ctlclient.Auth,
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
	rootCmd.AddCommand(&authCmd)
	return &rootCmd, nil, nil
}
