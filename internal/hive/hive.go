package hive

import (
	"context"
	"fmt"

	pb "github.com/pikvm/cloud-api/hive_for_agent"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/config/vars"
	"github.com/pikvm/kvmd-cloud/internal/util"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type HiveConnection struct {
	Ctx        context.Context
	Addr       string
	GrpcConn   *grpc.ClientConn
	HiveClient pb.HiveForAgentClient
}

var (
	hiveConnection *HiveConnection = nil
)

func Dial(ctx context.Context, group *errgroup.Group) (*pb.AvailableProxies, error) {
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
	c := pb.NewHiveForAgentClient(conn)
	if _, err := c.RegisterAgent(ctx, &pb.AgentInfo{
		Name: config.Cfg.AgentName,
	}); err != nil {
		return nil, err
	}
	log.Debugf("connected to hive %s", addr)
	proxies, err := c.GetAvailableProxies(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	hiveConnection = &HiveConnection{
		Ctx:        ctx,
		Addr:       addr,
		GrpcConn:   conn,
		HiveClient: c,
	}
	return proxies, nil
}
