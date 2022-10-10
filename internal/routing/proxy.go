package routing

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pikvm/kvmd-cloud/internal/config"
	log "github.com/sirupsen/logrus"
	"github.com/xornet-sl/gjrpc"
)

var (
	rpc *gjrpc.Rpc = nil
)

// proxy side requests a new connection
func requestConnectionRpc(handler *gjrpc.ApiHandler, cid string, connectTo string) error {
	_, err := CreateConnection(handler, cid, connectTo)
	return err
}

// routed connection lost on proxy's side
func connectionClosedNotification(cid string) {
	if connection := GetConnection(cid); connection != nil {
		connection.Close()
	}
}

// incoming data from proxy
func dataNotification(cid string, data string) {
	connection := GetConnection(cid)
	if connection == nil {
		log.Errorf("Unknown connection %s", cid)
		return
	}
}

func Serve(ctx context.Context) error {
	// TODO: dynamic proxies

	rpc = gjrpc.NewRpc(ctx)
	defer func() {
		rpc = nil
	}()

	rpc.RegisterMethod("requestConnection", requestConnectionRpc)
	rpc.RegisterMethod("connectionClosed", connectionClosedNotification)
	rpc.RegisterMethod("data", dataNotification)

	rpc.SetOnNewHandlerCallback(func(handler *gjrpc.ApiHandler) error {
		log.Debugf("connected to proxy %s", handler.Conn.RemoteAddr().String())
		go func() {
			err := handler.Call(handler.Ctx, nil, "registerAgent", config.Cfg.AgentName)
			if err != nil {
				log.WithError(err).Errorf("unable to register on proxy %s", handler.Conn.RemoteAddr().String())
				handler.Conn.Close()
			}
		}()
		return nil
	})

	return rpc.DialAndServe(fmt.Sprintf("ws://%s", config.Cfg.ProxyAddress), http.Header{}, nil)
}
