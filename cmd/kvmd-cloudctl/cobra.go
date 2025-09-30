package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/pikvm/kvmd-cloud/cmd/kvmd-cloudctl/ctl_client"
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
	RunE:  ctl_client.RequestStatus,
}

var setupCmd = cobra.Command{
	Use:   "setup",
	Short: "Authorize in cloud and configure agent",
	RunE:  Setup,
}

func init() {
	setupCmd.Flags().Bool("ask-token", false, "Ask for token interactively. Do not perform automatic token bootstrapping")
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
	rootCmd.AddCommand(&setupCmd)
	return &rootCmd, nil, nil
}
