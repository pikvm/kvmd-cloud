package routing

import (
	"encoding/base64"
	"net"
	"sync"

	"github.com/apex/log"
	"github.com/xornet-sl/gjrpc"
)

type Connection struct {
	Cid          string
	ConnectTo    string
	InnerConn    net.Conn
	ToInnerChan  chan string
	ProxyHandler *gjrpc.ApiHandler
}

var (
	connections   map[string]*Connection = nil
	connectionsMu sync.RWMutex
)

func GetConnection(cid string) *Connection {
	connectionsMu.RLock()
	defer connectionsMu.RUnlock()
	if connection, ok := connections[cid]; ok {
		return connection
	}
	return nil
}

func CreateConnection(apiHandler *gjrpc.ApiHandler, cid string, connectTo string) (*Connection, error) {
	var conn net.Conn
	var err error
	if connectTo[0] == '/' {
		conn, err = net.Dial("unix", connectTo)
	} else {
		conn, err = net.Dial("tcp", connectTo)
	}
	if err != nil {
		return nil, err
	}
	connection := &Connection{
		Cid:          cid,
		ConnectTo:    connectTo,
		InnerConn:    conn,
		ToInnerChan:  make(chan string, 1024), // TODO: congestion control
		ProxyHandler: apiHandler,
	}
	connectionsMu.Lock()
	defer connectionsMu.Unlock()
	connections[cid] = connection

	// Proxy -> Inner
	go func() {
		for {
			select {
			case <-connection.ProxyHandler.Ctx.Done():
				return // FIXME:
			case data, ok := <-connection.ToInnerChan:
				if !ok {
					return // FIXME:
				}
				dataBin, err := base64.StdEncoding.DecodeString(data)
				if err != nil {
					log.WithError(err).Errorf("base64 decode error")
				}
				if _, err := conn.Write(dataBin); err != nil {
					log.WithError(err).Errorf("unable to send data to inner connection")
					// TODO: shutdown logic
				}
			}
		}
	}()
	// Inner -> Proxy
	go func() {
		buff := make([]byte, 2048)
		for {
			n, err := conn.Read(buff)
			if err != nil {
				return // TODO:
			}
			apiHandler.Notify(
				apiHandler.Ctx,
				"data",
				connection.Cid,
				base64.StdEncoding.EncodeToString(buff[:n]),
			)
		}
	}()

	return connection, nil
}
