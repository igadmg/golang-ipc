package ipc

import "time"

const version = 2 // ipc package version

const (
	minMsgSize        = 1024
	defaultMaxMsgSize = 3145728 // 3Mb  - Maximum bytes allowed for each message
	defaultRetryTimer = time.Duration(200 * time.Millisecond)
)

var (
	defaultServerConfig = ServerConfig{
		SocketBasePath:    defaultSocketBasePath,
		Timeout:           0,
		MaxMsgSize:        defaultMaxMsgSize,
		Encryption:        true,
		UnmaskPermissions: false,
	}

	defaultClientConfig = ClientConfig{
		SocketBasePath: defaultSocketBasePath,
		Timeout:        0,
		RetryTimer:     defaultRetryTimer,
		MaxMsgSize:     defaultMaxMsgSize,
		Encryption:     true,
	}
)
