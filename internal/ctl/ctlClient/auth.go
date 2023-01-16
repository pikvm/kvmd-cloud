package ctlclient

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/hive"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"google.golang.org/protobuf/types/known/emptypb"
	"gopkg.in/yaml.v2"
)

const (
	authFilepath = "/etc/kvmd/cloud/auth.yaml"
)

func Auth(cmd *cobra.Command, args []string) error {
	fmt.Println("Input authorization data:")
	var agentName string
	fmt.Print("Agent name: ")
	if _, err := fmt.Scanln(&agentName); err != nil {
		return err
	}
	fmt.Print("Authorization token: ")
	b, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return err
	}
	token := string(b)

	config.Cfg.AgentName = agentName
	config.Cfg.AuthToken = token

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	conn, err := hive.Dial(ctx)
	if err != nil {
		return err
	}
	defer conn.GrpcConn.Close()
	if _, err = conn.HiveClient.AuthCheck(ctx, &emptypb.Empty{}); err != nil {
		return err
	}

	log.Info("Authorization successful")

	authFileContent := struct {
		AgentName string `yaml:"agent_name"`
		AuthToken string `yaml:"auth_token"`
	}{
		AgentName: agentName,
		AuthToken: token,
	}
	out, err := yaml.Marshal(&authFileContent)
	if err != nil {
		return err
	}
	if err = os.WriteFile(authFilepath, out, 0644); err != nil {
		return err
	}

	log.Info("Authorization information saved")

	return nil
}
