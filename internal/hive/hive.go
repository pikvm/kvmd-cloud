package hive

import (
	"context"
	"fmt"

	hive_pb "github.com/pikvm/cloud-api/hive_for_agent"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/config/vars"
	"github.com/pikvm/kvmd-cloud/internal/util"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type HiveConnection struct {
	Ctx        context.Context
	Addr       string
	GrpcConn   *grpc.ClientConn
	HiveClient hive_pb.HiveForAgentClient
	Events     HiveEventsChannel
}

var (
	hiveConnection *HiveConnection = nil
)

func Dial(ctx context.Context) (*HiveConnection, error) {
	if len(config.Cfg.Hive.Endpoints) == 0 {
		return nil, fmt.Errorf("hive endpoints not specified")
	}
	addr := config.Cfg.Hive.Endpoints[0]

	auth_md := map[string]string{
		"agent_uuid": vars.InstanceUUID,
		"agent_name": config.Cfg.AgentName,
	}
	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(util.NewInsecureRPCCred(auth_md, config.Cfg.AuthToken)),
	)
	if err != nil {
		return nil, err
	}
	c := hive_pb.NewHiveForAgentClient(conn)
	log.Debugf("connected to hive %s", addr)
	hiveConnection = &HiveConnection{
		Ctx:        ctx,
		Addr:       addr,
		GrpcConn:   conn,
		HiveClient: c,
		Events: HiveEventsChannel{
			Stream:          nil,
			SendEventsQueue: make(chan *EventSendPacket),
		},
	}

	return hiveConnection, nil
}

func CertbotAdd(ctx context.Context, domainName string, txt string) error {
	if hiveConnection == nil {
		return fmt.Errorf("not connected to hive")
	}
	_, err := hiveConnection.HiveClient.CertbotAdd(ctx, &hive_pb.CertbotDomainName{
		DomainName: domainName,
		Txt:        txt,
	})
	return err
}

func CertbotDel(ctx context.Context, domainName string) error {
	if hiveConnection == nil {
		return fmt.Errorf("not connected to hive")
	}
	_, err := hiveConnection.HiveClient.CertbotDel(ctx, &hive_pb.CertbotDomainName{
		DomainName: domainName,
	})
	return err
}
