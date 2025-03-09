package ipc

import (
	"bufio"
	"errors"
	"fmt"
	log "github.com/igadmg/golang-ipc/ipclogging"
	"io"
	"time"
)

// StartServer - starts the ipc server.
//
// ipcName - is the name of the unix socket or named pipe that will be created, the client needs to use the same name
func StartServer(ipcName string, config *ServerConfig) (*Server, error) {
	err := checkIpcName(ipcName)
	if err != nil {
		return nil, err
	}

	s := &Server{
		Name:     ipcName,
		status:   NotConnected,
		incoming: make(chan *Message),
		outgoing: make(chan *Message),
	}

	if config == nil {
		s.conf = DefaultServerConfig
	} else {
		s.conf = *config
	}

	if s.conf.Timeout < 0 {
		s.conf.Timeout = DefaultServerConfig.Timeout
	}
	if s.conf.MaxMsgSize < minMsgSize {
		s.conf.MaxMsgSize = DefaultServerConfig.MaxMsgSize
	}
	if s.conf.SocketBasePath == "" {
		s.conf.SocketBasePath = DefaultServerConfig.SocketBasePath
	}

	err = s.runServer()

	return s, err
}

func (s *Server) acceptClientConnectionsLoop() {
	for {
		conn, err := s.listen.Accept()
		if err != nil {
			break
		}

		if s.status == Listening || s.status == Disconnected {
			s.conn = conn

			err2 := s.serverHandshakeWithClient()
			if err2 != nil {
				s.incoming <- &Message{Err: err2, MsgType: ConnectionError}
				s.status = Error
				s.listen.Close()
				s.conn.Close()
			} else {
				go s.serverReadDataFromConnectionToIncomingChannel()
				go s.serverWriteDataFromOutgoingChannelToConnection()

				s.status = Connected
				log.Debugln("server sending initial IpcInternal msg")
				s.incoming <- &Message{Status: s.status.String(), MsgType: IpcInternal}
				s.connected <- true
			}
		}
	}
}

// func (s *Server) connectionTimer() error {
// 	if s.conf.Timeout != 0 {
// 		ticker := time.NewTicker(s.conf.Timeout)
// 		defer ticker.Stop()

// 		select {

// 		case <-s.connChannel:
// 			return nil
// 		case <-ticker.C:
// 			s.listen.Close()
// 			return errors.New("Timed out waiting for client to connect")
// 		}
// 	}

// 	select {

// 	case <-s.connChannel:
// 		return nil
// 	}

// }

func (s *Server) serverReadDataFromConnectionToIncomingChannel() {
	bLen := make([]byte, 4)

	for {
		res := s.readDataFromConnection(bLen)
		if !res {
			s.conn.Close()
			break
		}

		mLen := bytesToInt(bLen)
		msgRecvd := make([]byte, mLen)
		res = s.readDataFromConnection(msgRecvd)
		if !res {
			s.conn.Close()
			break
		}

		if s.conf.Encryption {
			msgFinal, err := decrypt(*s.enc.cipher, msgRecvd)
			if err != nil {
				s.incoming <- &Message{Err: err, MsgType: IpcInternal}
				continue
			}

			if bytesToInt(msgFinal[:4]) == IpcCtrl {
				//  type 0 = control message
				log.Debugf("server.serverReadDataFromConnectionToIncomingChannel() msgFinal HandshakeOk: %d", bytesToInt(msgFinal[:4])) // TODO
			} else {
				s.incoming <- &Message{Data: msgFinal[4:], MsgType: bytesToMsgType(msgFinal[:4])}
			}

		} else {
			if bytesToInt(msgRecvd[:4]) == IpcCtrl {
				//  type 0 = control message
				log.Debugf("server.serverReadDataFromConnectionToIncomingChannel() msgRecvd HandshakeOk: %d", bytesToInt(msgRecvd[:4])) // TODO
			} else {
				s.incoming <- &Message{Data: msgRecvd[4:], MsgType: bytesToMsgType(msgRecvd[:4])}
			}
		}
	}
}

func (s *Server) readDataFromConnection(buff []byte) bool {
	_, err := io.ReadFull(s.conn, buff)
	if err != nil {
		if s.status == Closing {
			s.status = Closed
			s.incoming <- &Message{Status: s.status.String(), MsgType: IpcInternal}
			s.incoming <- &Message{Err: errors.New("server has closed the connection"), MsgType: IpcInternal}
			return false
		}

		if err == io.EOF {
			s.status = Disconnected
			s.incoming <- &Message{Status: s.status.String(), MsgType: IpcInternal}
			return false
		}
	}

	return true
}

//func (s *Server) reConnect() {
//	s.status = ReConnecting
//	s.received <- &Message{Status: s.status.String(), MsgType: Internal}
//
//	err := s.connectionTimer()
//	if err != nil {
//		s.status = Timeout
//		s.received <- &Message{Status: s.status.String(), MsgType: Internal}
//
//		s.received <- &Message{err: err, MsgType: -2}
//	}
//}

// Read - blocking function, reads each message recieved
// if MsgType is a negative number its an internal message
func (s *Server) Read() (*Message, error) {
	m, ok := <-s.incoming
	if !ok {
		return nil, errors.New("server the received channel has been closed")
	}

	if m.Err != nil {
		return nil, m.Err
	}

	return m, nil
}

// Write - writes a message to the ipc connection
// msgType - denotes the type of data being sent. 0 is a reserved type for internal messages and errors.
func (s *Server) Write(msgType MsgType, message []byte) error {
	if msgType == IpcCtrl {
		return errors.New(fmt.Sprintf("server message type %d is reserved", IpcCtrl))
	}

	mlen := len(message)

	if mlen > s.conf.MaxMsgSize {
		return errors.New("server message exceeds maximum message length")
	}

	if s.status == Connected {
		s.outgoing <- &Message{MsgType: msgType, Data: message}
	} else {
		return errors.New(s.status.String())
	}

	return nil
}

func (s *Server) serverWriteDataFromOutgoingChannelToConnection() {
	var err error
	for {
		msg, ok := <-s.outgoing
		if !ok {
			break
		}

		toSend := msg.MsgType.toBytes()
		writer := bufio.NewWriter(s.conn)

		if s.conf.Encryption {
			toSend = append(toSend, msg.Data...)
			toSend, err = encrypt(*s.enc.cipher, toSend)
			if err != nil {
				log.Debugln("server error encrypting data", err)

				continue
			}
		} else {
			toSend = append(toSend, msg.Data...)
		}

		writer.Write(intToBytes(len(toSend)))
		writer.Write(toSend)
		err = writer.Flush()
		if err != nil {
			log.Debugln("server error flushing data", err)

			continue
		}

		time.Sleep(2 * time.Millisecond)
	}
}

// Status - returns the current connection status
func (s *Server) Status() Status {
	return s.status
}

// Close - closes the connection
func (s *Server) Close() {
	s.status = Closing

	if s.listen != nil {
		s.listen.Close()
	}

	if s.conn != nil {
		s.conn.Close()
	}

	if s.incoming != nil {
		s.incoming <- &Message{Status: s.status.String(), MsgType: IpcInternal}
		s.incoming <- &Message{Err: errors.New("server has closed the connection"), MsgType: ConnectionError}

		close(s.incoming)
	}

	if s.outgoing != nil {
		close(s.outgoing)
	}
}
