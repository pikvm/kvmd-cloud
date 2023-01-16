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

var certbotAddCmd = cobra.Command{
	Use:   "certbotAdd <domainName> <txt>",
	Short: "Adds DNS TXT record for certbot authorization challenge",
	RunE:  ctlclient.CertbotAdd,
}

var certbotDelCmd = cobra.Command{
	Use:   "certbotDel <domainName>",
	Short: "Cleanup DNS TXT record after certbot authorization challenge",
	RunE:  ctlclient.CertbotDel,
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
	rootCmd.AddCommand(&certbotAddCmd)
	rootCmd.AddCommand(&certbotDelCmd)
	return &rootCmd, nil, nil
}
