package ctlclient

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/pikvm/kvmd-cloud/internal/ctl"
)

func RequestStatus(cmd *cobra.Command, args []string) error {
	var status ctl.ApplicationStatusResponse
	err := DoUnixRequestJSON(cmd.Context(), "GET", "/status", nil, &status)
	log.Infof("%+v", status)
	return err
}
