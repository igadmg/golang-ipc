package ipc

import (
	"crypto/cipher"
	"net"
	"time"
)

// Server - holds the details of the server connection & config.
type Server struct {
	Name      string
	listen    net.Listener
	conn      net.Conn
	status    Status
	connected chan bool // ?
	received  chan (*Message)
	sent      chan (*Message)
	enc       *encryption
	conf      ServerConfig
}

// Client - holds the details of the client connection and config.
type Client struct {
	Name     string
	conn     net.Conn
	status   Status
	received chan (*Message)
	sent     chan (*Message)
	enc      *encryption
	conf     ClientConfig
}

// Message - contains the  received message
type Message struct {
	Err     error  // details of any error
	MsgType int    // 0 = reserved , -1 is an internal message (disconnection or error etc), all messages recieved will be > 0
	Data    []byte // message data received
	Status  string // the status of the connection
}

// Status - Status of the connection
type Status int

const (
	// NotConnected - 0
	NotConnected Status = iota
	// Listening - 1
	Listening
	// Connecting - 2
	Connecting
	// Connected - 3
	Connected
	// ReConnecting - 4
	ReConnecting
	// Closed - 5
	Closed
	// Closing - 6
	Closing
	// Error - 7
	Error
	// Timeout - 8
	Timeout
	// Disconnected - 9
	Disconnected
)

// ServerConfig - used to pass configuration overrides to ServerStart()
type ServerConfig struct {
	SocketBasePath    string
	Timeout           time.Duration
	MaxMsgSize        int
	Encryption        bool
	UnmaskPermissions bool
}

// ClientConfig - used to pass configuration overrides to ClientStart()
type ClientConfig struct {
	SocketBasePath string
	Timeout        time.Duration
	RetryTimer     time.Duration
	MaxMsgSize     int
	Encryption     bool
}

// Encryption - encryption settings
type encryption struct {
	keyExchange string
	encryption  string
	cipher      *cipher.AEAD
}
