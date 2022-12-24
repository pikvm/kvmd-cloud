package proxy

import (
	"errors"
	"io"
	"net"
	"sync"
	"syscall"

	pb "github.com/pikvm/cloud-api/proxy_for_agent"
	log "github.com/sirupsen/logrus"
)

type Connection struct {
	ProxyConnection *ProxyConnection
	Cid             string
	ConnectTo       string
	InnerConn       net.Conn
	ProxyStream     pb.ProxyForAgent_ConnectionChannelClient
	// ToInnerChan     chan []byte
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
	return connections[cid]
}

func startNewConnection(pconn *ProxyConnection, connDescr *pb.ProxyEvent_NewConnection) error {
	var (
		conn net.Conn
		err  error
	)
	cid := connDescr.GetCid()
	connectTo := connDescr.GetConnectTo()
	if connectTo[0] == '/' {
		conn, err = net.Dial("unix", connectTo)
	} else {
		conn, err = net.Dial("tcp", connectTo)
	}
	if err != nil {
		return err
	}

	stream, err := pconn.ProxyClient.ConnectionChannel(pconn.Ctx)
	if err != nil {
		return err
	}

	// Send initial metadata
	if err = stream.Send(&pb.ConnectionMessage{
		Type: pb.ConnectionMessage_MSGTYPE_METADATA,
		Body: &pb.ConnectionMessage_Metadata_{
			Metadata: &pb.ConnectionMessage_Metadata{
				Cid: cid,
			},
		},
	}); err != nil {
		stream.CloseSend()
		return err
	}

	connection := &Connection{
		ProxyConnection: pconn,
		Cid:             cid,
		ConnectTo:       connectTo,
		InnerConn:       conn,
		//
		// ToInnerChan: make(chan []byte, 1024), // TODO: congestion control
		ProxyStream: stream,
	}
	connectionsMu.Lock()
	defer connectionsMu.Unlock()
	connections[cid] = connection

	log.Debugf("Connection cid:%s created", connection.Cid)

	// Proxy -> Inner
	go func() {
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				// Connection closed on proxy's side
				connection.Close()
				return
			}
			if err != nil {
				log.WithError(err).Errorf("error while getting data from proxy")
				connection.Close()
				return
			}
			chunk := msg.GetChunk()
			if _, err := connection.InnerConn.Write(chunk); err != nil {
				log.WithError(err).Errorf("unable to send data to inner connection")
				connection.Close()
				// apiHandler.Notify(apiHandler.Ctx, "connectionClosed", connection.Cid)
				return
			}
		}
	}()
	// Inner -> Proxy
	go func() {
		readCloserChan := make(chan struct{})
		go func() {
			select {
			case <-pconn.Ctx.Done():
				connection.Close()
				return
			case <-readCloserChan:
				return
			}
		}()
		defer close(readCloserChan)
		buff := make([]byte, 2048)
		for {
			n, err := connection.InnerConn.Read(buff)
			if isNetConnClosedErr(err) {
				return
			} else if err != nil {
				log.WithError(err).Errorf("error reading from inner connection")
				connection.Close()
				return
			}
			if err = connection.ProxyStream.Send(&pb.ConnectionMessage{
				Body: &pb.ConnectionMessage_Chunk{
					Chunk: buff[:n],
				},
			}); err != nil {
				log.WithError(err).Errorf("unable to send data to proxy")
				connection.Close()
				return
			}
		}
	}()

	return nil
}

func (this *Connection) Close() {
	this.InnerConn.Close()
	connectionsMu.Lock()
	defer connectionsMu.Unlock()
	delete(connections, this.Cid)
	log.Debugf("Connection cid:%s closed", this.Cid)
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
