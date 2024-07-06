//go:build linux || darwin
// +build linux darwin

package ipc

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var defaultSocketBasePath = "/tmp/"

// Server create a unix socket and start listening connections - for unix and linux
func (s *Server) run() error {
	socketPath := filepath.Join(s.conf.SocketBasePath, s.name+".sock")

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

	go s.acceptLoop()
	s.status = Listening
	//sc.received <- &Message{Status: sc.status.String(), MsgType: -1}
	//sc.connChannel = make(chan bool)

	return nil
}

// Client connect to the unix socket created by the server -  for unix and linux
func (c *Client) dial() error {
	socketPath := filepath.Join(c.conf.SocketBasePath, c.Name+".sock")
	startTime := time.Now()

	for {
		if c.conf.Timeout != 0 {
			if time.Since(startTime) > c.conf.Timeout {
				c.status = Closed
				return errors.New("timed out trying to connect")
			}
		}

		conn, err := net.Dial("unix", socketPath)
		if err != nil {
			if strings.Contains(err.Error(), "connect: no such file or directory") {
			} else if strings.Contains(err.Error(), "connect: connection refused") {
			} else {
				c.received <- &Message{Err: err, MsgType: -1}
			}
		} else {
			c.conn = conn

			err = c.handshake()
			if err != nil {
				return err
			}

			return nil
		}

		time.Sleep(c.conf.RetryTimer * time.Second)
	}
}
