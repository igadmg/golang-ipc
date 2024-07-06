package ipc

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// 1st message sent from the server
// byte 0 = protocal version no.
// byte 1 = whether encryption is to be used - 0 no , 1 = encryption
func (s *Server) handshake() error {
	err := s.one()
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

func (s *Server) one() error {
	buff := make([]byte, 2)
	buff[0] = byte(version)

	if s.conf.Encryption {
		buff[1] = byte(1)
	} else {
		buff[1] = byte(0)
	}

	_, err := s.conn.Write(buff)
	if err != nil {
		return errors.New("unable to send handshake ")
	}

	recv := make([]byte, 1)
	_, err = s.conn.Read(recv)
	if err != nil {
		return errors.New("failed to received handshake reply")
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

	return errors.New("other error - handshake failed")
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
		return errors.New("unable to send max message length ")
	}

	reply := make([]byte, 1)
	_, err = s.conn.Read(reply)
	if err != nil {
		return errors.New("did not received message length reply")
	}

	return nil
}

// 1st message received by the client
func (c *Client) handshake() error {
	err := c.one()
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

func (c *Client) one() error {
	recv := make([]byte, 2)
	_, err := c.conn.Read(recv)
	if err != nil {
		return errors.New("failed to received handshake message")
	}

	if recv[0] != version {
		c.handshakeSendReply(1)
		return errors.New("server has sent a different version number")
	}

	if recv[1] != 1 && c.conf.Encryption {
		c.handshakeSendReply(2)
		return errors.New("server tried to connect without encryption")
	}

	if recv[1] == 0 {
		c.conf.Encryption = false
	} else {
		c.conf.Encryption = true
	}

	c.handshakeSendReply(0) // 0 is ok
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
		return errors.New("failed to received max message length 1")
	}

	var msgLen uint32
	binary.Read(bytes.NewReader(buff), binary.BigEndian, &msgLen) // message length

	buff = make([]byte, int(msgLen))
	_, err = c.conn.Read(buff)
	if err != nil {
		return errors.New("failed to received max message length 2")
	}
	var buff2 []byte
	if c.conf.Encryption {
		buff2, err = decrypt(*c.enc.cipher, buff)
		if err != nil {
			return errors.New("failed to received max message length 3")
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

func (c *Client) handshakeSendReply(result byte) {
	buff := make([]byte, 1)
	buff[0] = result

	c.conn.Write(buff)
}
