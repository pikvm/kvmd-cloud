package ctlclient

import (
	"fmt"

	"github.com/pikvm/kvmd-cloud/internal/ctl"
	"github.com/spf13/cobra"
)

func CertbotAdd(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		cmd.Usage()
		return fmt.Errorf("domainName and/or txt are missing")
	}
	domainName := ctl.CertbotDomainName{
		DomainName: args[0],
		TXT:        args[1],
	}
	var response ctl.CertbotResponse
	if err := DoUnixRequestJSON(cmd.Context(), "POST", "/certbotAdd", domainName, &response); err != nil {
		return err
	}
	if !response.Ok {
		return fmt.Errorf(response.Error)
	}
	return nil
}

func CertbotDel(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		cmd.Usage()
		return fmt.Errorf("domainName is missing")
	}
	domainName := ctl.CertbotDomainName{
		DomainName: args[0],
	}
	var response ctl.CertbotResponse
	if err := DoUnixRequestJSON(cmd.Context(), "POST", "/certbotDel", domainName, &response); err != nil {
		return err
	}
	if !response.Ok {
		return fmt.Errorf(response.Error)
	}
	return nil
}
