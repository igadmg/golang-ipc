package ipc

import (
	"crypto/cipher"
	"net"
	"time"
)

// Server - holds the details of the server connection & config.
type Server struct {
	Name      string
	listen    net.Listener // listener for connections
	conn      net.Conn     // socket/namedPipe connection to a client
	status    Status
	connected chan bool // server has connected client(s)
	incoming  chan *Message
	outgoing  chan *Message
	enc       *encryption
	conf      ServerConfig
}

// Client - holds the details of the client connection and config.
type Client struct {
	Name     string
	conn     net.Conn // socket/namedPipe connection to server
	status   Status
	incoming chan *Message
	outgoing chan *Message
	enc      *encryption
	conf     ClientConfig
}

// Message - contains the  received message
type Message struct {
	Err     error   // details of any error
	MsgType MsgType // 0 = reserved , -1 is an internal message (disconnection or error etc), all messages recieved will be > 0
	Data    []byte  // message data received
	Status  string  // the status of the connection
}

// Status - Status of the connection
type Status int

const (
	NotConnected Status = iota // 0
	Listening                  // 1
	Connecting                 // 2
	Connected                  // 3
	ReConnecting               // 4
	Closed                     // 5
	Closing                    // 6
	Error                      // 7
	Timeout                    // 8
	Disconnected               // 9
)

type MsgType int

const (
	ConnectionError MsgType = -2       // -2
	IpcInternal             = iota - 2 // -1
	IpcCtrl                            // 0
	String                             // 1
	Int                                // 2
	Float                              // 3
	Struct                             // 4
	Custom                             // 5
)

type HandshakeResult byte

const (
	HandshakeOk              HandshakeResult = iota // 0
	IpcVersionMismatch                              // 1
	ClientEncryptedServerNot                        // 2
	ServerEncryptedClientNot                        // 3
)

type Encryption byte

const (
	Plain Encryption = iota
	Encrypted
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
