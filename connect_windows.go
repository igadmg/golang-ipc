//go:build windows
// +build windows

package ipc

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
)

var defaultSocketBasePath = `\\.\pipe\`

// Server function
// Create the named pipe (if it doesn't already exist) and start listening for a client to connect.
// when a client connects and connection is accepted the read function is called on a go routine.
func (s *Server) run() error {
	socketPath := filepath.Join(s.conf.SocketBasePath, s.Name)
	var config *winio.PipeConfig

	if s.conf.UnmaskPermissions {
		config = &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;AU)"}
	}

	listen, err := winio.ListenPipe(socketPath, config)
	if err != nil {
		return err
	}

	s.listen = listen
	s.status = Listening
	go s.acceptLoop()

	return nil
}

// Client function
// dial - attempts to connect to a named pipe created by the server
func (c *Client) dial() error {
	socketPath := filepath.Join(c.conf.SocketBasePath, c.name)
	startTime := time.Now()

	for {
		if c.conf.Timeout != 0 {
			if time.Since(startTime) > c.conf.Timeout {
				c.status = Closed
				return errors.New("timed out trying to connect")
			}
		}

		pn, err := winio.DialPipe(socketPath, nil)
		if err != nil {
			if strings.Contains(err.Error(), "the system cannot find the file specified.") == true {
			} else {
				return err
			}
		} else {
			c.conn = pn

			err = c.handshake()
			if err != nil {
				return err
			}

			return nil
		}

		time.Sleep(c.conf.RetryTimer * time.Second)
	}
}
