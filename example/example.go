package main

import (
	ipc "github.com/igadmg/golang-ipc"
	log "github.com/igadmg/golang-ipc/ipclogging"
	"time"
)

func main() {
	log.DoDebug = true
	go server()
	time.Sleep(5000 * time.Nanosecond) // client shouldn't connect before server listens

	clientConfig := &ipc.ClientConfig{
		SocketBasePath: ipc.DefaultClientConfig.SocketBasePath,
		Timeout:        ipc.DefaultClientConfig.Timeout,
		RetryTimer:     ipc.DefaultClientConfig.RetryTimer,
		MaxMsgSize:     ipc.DefaultClientConfig.MaxMsgSize,
		Encryption:     false,
	}
	c, err := ipc.StartClient("example1", clientConfig)
	if err != nil {
		log.Println(err)
		return
	}

	for {
		message, err := c.Read()

		if err == nil {
			if message.MsgType == ipc.IpcInternal {
				log.Printf("client received IpcInternal: %s", ipc.StatusString(c.Status()))

				if message.Status == "Reconnecting" {
					c.Close()
					return
				}
			} else {
				log.Printf("client received Msg: %s - Message type: %d", string(message.Data), message.MsgType)
				internalReply := "reply from example client: client is connected to server"
				c.Write(ipc.Custom, []byte(internalReply))
				log.Printf("client sent Msg: '%s'", internalReply)
			}
		} else {
			log.Println(err)
			break
		}
	}
}

func server() {
	serverConfig := &ipc.ServerConfig{
		SocketBasePath:    ipc.DefaultServerConfig.SocketBasePath,
		Timeout:           ipc.DefaultServerConfig.Timeout,
		MaxMsgSize:        ipc.DefaultServerConfig.MaxMsgSize,
		Encryption:        false,
		UnmaskPermissions: false,
	}
	s, err := ipc.StartServer("example1", serverConfig)
	if err != nil {
		log.Println("server error", err)
		return
	}

	log.Printf("server status: %s", ipc.StatusString(s.Status()))

	for {
		message, err := s.Read()

		if err == nil {
			if message.MsgType == ipc.IpcInternal {
				if message.Status == "Connected" {
					log.Printf("server received IpcInternal: %s", ipc.StatusString(s.Status()))
					internalReply := "server reply: server is connected to client"
					s.Write(ipc.String, []byte(internalReply))
					log.Printf("server sent Msg: '%s'", internalReply)
				} else {
					log.Printf("server received IpcInternal: %s", string(message.Data))
				}
			} else {
				log.Printf("server received Msg: '%s' - Message type: %d", string(message.Data), message.MsgType)
				s.Close()
				return
			}
		} else {
			break
		}
	}
}
