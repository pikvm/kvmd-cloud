package ctl_client

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"github.com/pikvm/kvmd-cloud/internal/ctl"
)

func BuildStatusCommand() *cli.Command {
	return &cli.Command{
		Name:   "status",
		Usage:  "kvmd-cloud status",
		Action: RequestStatus,
	}
}

func RequestStatus(ctx context.Context, cmd *cli.Command) error {
	logger := log.Logger

	var status ctl.ApplicationStatusResponse
	err := DoUnixRequestJSON(ctx, "GET", "/status", nil, &status)
	logger.Info().Msgf("%+v", status)
	return err
}
