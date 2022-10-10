package routing

import (
	"encoding/base64"
	"errors"
	"io"
	"net"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"
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

func init() {
	connections = make(map[string]*Connection)
}

func GetConnection(cid string) *Connection {
	connectionsMu.RLock()
	defer connectionsMu.RUnlock()
	if connection, ok := connections[cid]; ok {
		return connection
	}
	return nil
}

func (this *Connection) Close() {
	this.InnerConn.Close()
	connectionsMu.Lock()
	defer connectionsMu.Unlock()
	delete(connections, this.Cid)
	log.Debugf("Connection cid:%s closed", this.Cid)
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
		Cid:       cid,
		ConnectTo: connectTo,
		InnerConn: conn,
		//
		ToInnerChan:  make(chan string, 1024), // TODO: congestion control
		ProxyHandler: apiHandler,
	}
	connectionsMu.Lock()
	defer connectionsMu.Unlock()
	connections[cid] = connection

	log.Debugf("Connection cid:%s created", connection.Cid)

	// Proxy -> Inner
	go func() {
		for {
			select {
			case <-connection.ProxyHandler.Ctx.Done():
				connection.Close()
				return
			case data, ok := <-connection.ToInnerChan:
				if !ok {
					// Connection closed on proxy's side
					connection.Close()
					return
				}
				dataBin, err := base64.StdEncoding.DecodeString(data)
				if err != nil {
					log.WithError(err).Errorf("base64 decode error")
				}
				if _, err := connection.InnerConn.Write(dataBin); err != nil {
					log.WithError(err).Errorf("unable to send data to inner connection")
					connection.Close()
					apiHandler.Notify(apiHandler.Ctx, "connectionClosed", connection.Cid)
					return
				}
			}
		}
	}()
	// Inner -> Proxy
	go func() {
		readCloserChan := make(chan struct{})
		go func() {
			select {
			case <-connection.ProxyHandler.Ctx.Done():
				connection.Close()
				return
			case <-readCloserChan:
				return
			}
		}()
		buff := make([]byte, 2048)
		for {
			n, err := connection.InnerConn.Read(buff)
			if isNetConnClosedErr(err) {
				close(readCloserChan)
				return
			} else if err != nil {
				log.WithError(err).Errorf("error reading from inner connection")
				close(readCloserChan)
				connection.Close()
				return
			}
			if err = apiHandler.Notify(
				apiHandler.Ctx,
				"data",
				connection.Cid,
				base64.StdEncoding.EncodeToString(buff[:n]),
			); err != nil {
				log.WithError(err).Errorf("unable to send data to proxy")
				close(readCloserChan)
				connection.Close()
				return
			}
		}
	}()

	return connection, nil
}

func isNetConnClosedErr(err error) bool {
	switch {
	case
		errors.Is(err, net.ErrClosed),
		errors.Is(err, io.EOF),
		errors.Is(err, syscall.EPIPE):
		return true
	default:
		return false
	}
}
