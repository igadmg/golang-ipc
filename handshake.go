package ipc

import (
	"bytes"
	"encoding/binary"
	"errors"
	log "github.com/igadmg/golang-ipc/ipclogging"
)

// 1st message outgoing from the server
// byte 0 = protocol ipcVersion number.
// byte 1 = whether encryption is to be used: 0 = not-encrypted , 1 = encrypted
func (s *Server) serverHandshakeWithClient() error {
	err := s.serverSendAndReceiveHandshakeMessage()
	if err != nil {
		return err
	}

	if s.conf.Encryption {
		err = s.startEncryption()
		if err != nil {
			return err
		}
	}

	err = s.msgLength()
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) serverSendAndReceiveHandshakeMessage() error {
	buff := make([]byte, 2)
	buff[0] = byte(ipcVersion)

	if s.conf.Encryption {
		buff[1] = byte(Encrypted)
	} else {
		buff[1] = byte(Plain)
	}

	_, err := s.conn.Write(buff)
	if err != nil {
		return errors.New("server unable to send handshake ")
	} else {
		log.Debugln("server ok sent handshake")
	}

	recv := make([]byte, 1)
	_, err = s.conn.Read(recv)
	if err != nil {
		return errors.New("server failed to received handshake reply")
	} else {
		log.Debugf("server ok received handshake: %d", recv[0])
	}

	switch result := recv[0]; result {
	case 0:
		return nil
	case 1:
		return errors.New("client has a different version number")
	case 2:
		return errors.New("client is enforcing encryption")
	case 3:
		return errors.New("server failed to get handshake reply")

	}

	return errors.New("server other error - handshake failed")
}

func (s *Server) startEncryption() error {
	shared, err := s.keyExchange()
	if err != nil {
		return err
	}

	gcm, err := createCipher(shared)
	if err != nil {
		return err
	}

	s.enc = &encryption{
		keyExchange: "ecdsa",
		encryption:  "AES-GCM-256",
		cipher:      gcm,
	}

	return nil
}

func (s *Server) msgLength() error {
	toSend := make([]byte, 4)
	buff := make([]byte, 4)
	binary.BigEndian.PutUint32(buff, uint32(s.conf.MaxMsgSize))

	if s.conf.Encryption {
		maxMsg, err := encrypt(*s.enc.cipher, buff)
		if err != nil {
			return err
		}

		binary.BigEndian.PutUint32(toSend, uint32(len(maxMsg)))
		toSend = append(toSend, maxMsg...)
	} else {
		binary.BigEndian.PutUint32(toSend, uint32(len(buff)))
		toSend = append(toSend, buff...)
	}

	_, err := s.conn.Write(toSend)
	if err != nil {
		return errors.New("server unable to send max message length")
	}

	reply := make([]byte, 1)
	_, err = s.conn.Read(reply)
	if err != nil {
		return errors.New("server did not received message length reply")
	}

	return nil
}

// 1st message incoming by the client
func (c *Client) clientHandshake() error {
	err := c.clientReceiveAndSendHandshakeMessage()
	if err != nil {
		return err
	}

	if c.conf.Encryption {
		err := c.startEncryption()
		if err != nil {
			return err
		}
	}

	err = c.msgLength()
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) clientReceiveAndSendHandshakeMessage() error {
	recv := make([]byte, 2)
	_, err := c.conn.Read(recv)
	if err != nil {
		return errors.New("client failed to received handshake message")
	} else {
		log.Debugf("client received handshake (version/encryption): (%d/%d)", recv[0], recv[1])
	}

	if recv[0] != ipcVersion {
		c.handshakeSendReply(IpcVersionMismatch)
		return errors.New("client server has sent a different ipcVersion number")
	}

	if recv[1] == byte(Plain) && c.conf.Encryption {
		c.handshakeSendReply(ClientEncryptedServerNot)
		return errors.New("client server communicates unencrypted/plain, client wants encrypted communication")
	}

	if recv[1] == byte(Plain) {
		c.conf.Encryption = false
	} else {
		c.conf.Encryption = true
	}

	log.Debugln("client sending handshakeOk")
	c.handshakeSendReply(HandshakeOk) // 0 is ok
	return nil
}

func (c *Client) startEncryption() error {
	shared, err := c.keyExchange()
	if err != nil {
		return err
	}

	gcm, err := createCipher(shared)
	if err != nil {
		return err
	}

	c.enc = &encryption{
		keyExchange: "ECDSA",
		encryption:  "AES-GCM-256",
		cipher:      gcm,
	}

	return nil
}

func (c *Client) msgLength() error {
	buff := make([]byte, 4)
	_, err := c.conn.Read(buff)
	if err != nil {
		return errors.New("client failed to received max message length 1")
	}

	var msgLen uint32
	binary.Read(bytes.NewReader(buff), binary.BigEndian, &msgLen) // message length

	buff = make([]byte, int(msgLen))
	_, err = c.conn.Read(buff)
	if err != nil {
		return errors.New("client failed to received max message length 2")
	}
	var buff2 []byte
	if c.conf.Encryption {
		buff2, err = decrypt(*c.enc.cipher, buff)
		if err != nil {
			return errors.New("client failed to received max message length 3")
		}
	} else {
		buff2 = buff
	}

	var maxMsgSize uint32
	binary.Read(bytes.NewReader(buff2), binary.BigEndian, &maxMsgSize) // message length

	c.conf.MaxMsgSize = int(maxMsgSize)
	c.handshakeSendReply(0)

	return nil
}

func (c *Client) handshakeSendReply(result HandshakeResult) {
	buff := make([]byte, 1)
	buff[0] = byte(result)

	c.conn.Write(buff)
}
