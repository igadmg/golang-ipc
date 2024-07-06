package ipc

import (
	"bufio"
	"errors"
	"io"
	"log"
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
		received: make(chan *Message),
		sent:     make(chan *Message),
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

	err = s.run()

	return s, err
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listen.Accept()
		if err != nil {
			break
		}

		if s.status == Listening || s.status == Disconnected {
			s.conn = conn

			err2 := s.handshake()
			if err2 != nil {
				s.received <- &Message{Err: err2, MsgType: -2}
				s.status = Error
				s.listen.Close()
				s.conn.Close()
			} else {
				go s.read()
				go s.write()

				s.status = Connected
				s.received <- &Message{Status: s.status.String(), MsgType: -1}
				s.connected <- true
			}
		}
	}
}

// func (sc *Server) connectionTimer() error {
// 	if sc.conf.Timeout != 0 {
// 		ticker := time.NewTicker(sc.conf.Timeout)
// 		defer ticker.Stop()

// 		select {

// 		case <-sc.connChannel:
// 			return nil
// 		case <-ticker.C:
// 			sc.listen.Close()
// 			return errors.New("Timed out waiting for client to connect")
// 		}
// 	}

// 	select {

// 	case <-sc.connChannel:
// 		return nil
// 	}

// }

func (s *Server) read() {
	bLen := make([]byte, 4)

	for {
		res := s.readData(bLen)
		if !res {
			s.conn.Close()

			break
		}

		mLen := bytesToInt(bLen)
		msgRecvd := make([]byte, mLen)
		res = s.readData(msgRecvd)
		if !res {
			s.conn.Close()

			break
		}

		if s.conf.Encryption {
			msgFinal, err := decrypt(*s.enc.cipher, msgRecvd)
			if err != nil {
				s.received <- &Message{Err: err, MsgType: -1}

				continue
			}

			if bytesToInt(msgFinal[:4]) == 0 {
				//  type 0 = control message
			} else {
				s.received <- &Message{Data: msgFinal[4:], MsgType: bytesToInt(msgFinal[:4])}
			}

		} else {
			if bytesToInt(msgRecvd[:4]) == 0 {
				//  type 0 = control message
			} else {
				s.received <- &Message{Data: msgRecvd[4:], MsgType: bytesToInt(msgRecvd[:4])}
			}
		}
	}
}

func (s *Server) readData(buff []byte) bool {
	_, err := io.ReadFull(s.conn, buff)
	if err != nil {
		if s.status == Closing {
			s.status = Closed
			s.received <- &Message{Status: s.status.String(), MsgType: -1}
			s.received <- &Message{Err: errors.New("server has closed the connection"), MsgType: -1}
			return false
		}

		if err == io.EOF {
			s.status = Disconnected
			s.received <- &Message{Status: s.status.String(), MsgType: -1}
			return false
		}
	}

	return true
}

//func (sc *Server) reConnect() {
//	sc.status = ReConnecting
//	sc.received <- &Message{Status: sc.status.String(), MsgType: -1}
//
//	err := sc.connectionTimer()
//	if err != nil {
//		sc.status = Timeout
//		sc.received <- &Message{Status: sc.status.String(), MsgType: -1}
//
//		sc.received <- &Message{err: err, MsgType: -2}
//	}
//}

// Read - blocking function, reads each message recieved
// if MsgType is a negative number its an internal message
func (s *Server) Read() (*Message, error) {
	m, ok := (<-s.received)
	if !ok {
		return nil, errors.New("the received channel has been closed")
	}

	if m.Err != nil {
		return nil, m.Err
	}

	return m, nil
}

// Write - writes a message to the ipc connection
// msgType - denotes the type of data being sent. 0 is a reserved type for internal messages and errors.
func (s *Server) Write(msgType int, message []byte) error {
	if msgType == 0 {
		return errors.New("message type 0 is reserved")
	}

	mlen := len(message)

	if mlen > s.conf.MaxMsgSize {
		return errors.New("message exceeds maximum message length")
	}

	if s.status == Connected {
		s.sent <- &Message{MsgType: msgType, Data: message}
	} else {
		return errors.New(s.status.String())
	}

	return nil
}

func (s *Server) write() {
	var err error
	for {
		m, ok := <-s.sent
		if !ok {
			break
		}

		toSend := intToBytes(m.MsgType)
		writer := bufio.NewWriter(s.conn)

		if s.conf.Encryption {
			toSend = append(toSend, m.Data...)
			toSend, err = encrypt(*s.enc.cipher, toSend)
			if err != nil {
				log.Println("error encrypting data", err)

				continue
			}
		} else {
			toSend = append(toSend, m.Data...)
		}

		writer.Write(intToBytes(len(toSend)))
		writer.Write(toSend)
		err = writer.Flush()
		if err != nil {
			log.Println("error flushing data", err)

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

	if s.received != nil {
		s.received <- &Message{Status: s.status.String(), MsgType: -1}
		s.received <- &Message{Err: errors.New("Server has closed the connection"), MsgType: -2}

		close(s.received)
	}

	if s.sent != nil {
		close(s.sent)
	}
}
