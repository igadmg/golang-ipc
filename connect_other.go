//go:build linux || darwin
// +build linux darwin

package ipc

import (
	"errors"
	log "github.com/igadmg/golang-ipc/ipclogging"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var defaultSocketBasePath = "/tmp/"
var defaultSocketExt = ".sock"

// Server create a unix socket and start listening connections - for unix and linux
func (s *Server) runServer() error {
	socketPath := filepath.Join(s.conf.SocketBasePath, s.Name+defaultSocketExt)

	if err := os.RemoveAll(socketPath); err != nil {
		return err
	}

	if s.conf.UnmaskPermissions {
		defer syscall.Umask(syscall.Umask(0))
	}

	listen, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}

	s.listen = listen
	s.status = Listening
	log.Debugln("server ok connected to socket ... now entering accept loop")
	go s.acceptClientConnectionsLoop()
	//sc.received <- &Message{Status: sc.status.String(), MsgType: Internal}
	//sc.connChannel = make(chan bool)

	return nil
}

// Client connect to the unix socket created by the server -  for unix and linux
func (c *Client) dial() error {
	socketPath := filepath.Join(c.conf.SocketBasePath, c.Name+defaultSocketExt)
	startTime := time.Now()

	for {
		if c.conf.Timeout != 0 {
			if time.Since(startTime) > c.conf.Timeout {
				c.status = Closed
				return errors.New("client timed out trying to connect")
			}
		}

		socketConnection, err := net.Dial("unix", socketPath)
		if err != nil {
			if strings.Contains(err.Error(), "client dial: no such file or directory") {
			} else if strings.Contains(err.Error(), "client dial: connection refused") {
			} else {
				c.incoming <- &Message{Err: err, MsgType: IpcInternal}
			}
		} else {
			c.conn = socketConnection

			log.Debugln("client connected to server socket ... now waiting for server handshake")
			err = c.clientHandshake()
			if err != nil {
				return err
			}

			return nil
		}

		time.Sleep(c.conf.RetryTimer * time.Second)
	}
}
