package ipc

import (
	"bufio"
	"errors"
	"fmt"
	log "github.com/igadmg/golang-ipc/ipclogging"
	"io"
	"strings"
)

// StartClient - start the ipc client.
// ipcName = is the name of the unix socket or named pipe that the client will try and connect to.
func StartClient(ipcName string, config *ClientConfig) (*Client, error) {
	err := checkIpcName(ipcName)
	if err != nil {
		return nil, err
	}

	c := &Client{
		Name:     ipcName,
		status:   NotConnected,
		incoming: make(chan *Message),
		outgoing: make(chan *Message),
	}

	if config == nil {
		c.conf = DefaultClientConfig
	} else {
		c.conf = *config
	}

	if c.conf.Timeout < 0 {
		c.conf.Timeout = DefaultClientConfig.Timeout
	}
	if c.conf.RetryTimer <= 0 {
		c.conf.RetryTimer = DefaultClientConfig.RetryTimer
	}
	if c.conf.SocketBasePath == "" {
		c.conf.SocketBasePath = DefaultClientConfig.SocketBasePath
	}

	go startClient(c)

	return c, nil
}

func startClient(c *Client) {
	c.status = Connecting
	log.Debugln("client sending initial IpcInternal msg")
	c.incoming <- &Message{Status: c.status.String(), MsgType: IpcInternal}

	err := c.dial()
	if err != nil {
		c.incoming <- &Message{Err: err, MsgType: IpcInternal}
		return
	}

	c.status = Connected
	c.incoming <- &Message{Status: c.status.String(), MsgType: IpcInternal}

	go c.clientReadDataFromConnectionToIncomingChannel()
	go c.clientWriteDataFromOutgoingChannelToConnection()
}

func (c *Client) clientReadDataFromConnectionToIncomingChannel() {
	bLen := make([]byte, 4)

	for {
		res := c.readData(bLen)
		if !res {
			break
		}

		mLen := bytesToInt(bLen)
		msg := make([]byte, mLen)
		res = c.readData(msg)
		if !res {
			break
		}

		if c.conf.Encryption {
			msgFinal, err := decrypt(*c.enc.cipher, msg)
			if err != nil {
				break
			}

			if bytesToInt(msgFinal[:4]) == 0 {
				//  type 0 = control message
			} else {
				c.incoming <- &Message{Data: msgFinal[4:], MsgType: bytesToMsgType(msgFinal[:4])}
			}
		} else {
			if bytesToInt(msg[:4]) == 0 {
				//  type 0 = control message
			} else {
				c.incoming <- &Message{Data: msg[4:], MsgType: bytesToMsgType(msg[:4])}
			}
		}
	}
}

func (c *Client) readData(buff []byte) bool {
	_, err := io.ReadFull(c.conn, buff)
	if err != nil {
		if strings.Contains(err.Error(), "EOF") { // the connection has been closed by the client.
			c.conn.Close()
			if c.status != Closing || c.status == Closed {
				go c.reconnect()
			}

			return false
		}

		if c.status == Closing {
			c.status = Closed
			c.incoming <- &Message{Status: c.status.String(), MsgType: IpcInternal}
			c.incoming <- &Message{Err: errors.New("client has closed the connection"), MsgType: -2}

			return false
		}

		// other serverReadDataFromConnectionToIncomingChannel error
		return false
	}

	return true
}

func (c *Client) reconnect() {
	c.status = ReConnecting
	c.incoming <- &Message{Status: c.status.String(), MsgType: IpcInternal}
	err := c.dial() // connect to the pipe
	if err != nil {
		if err.Error() == "client timed out trying to connect" {
			c.status = Timeout
			c.incoming <- &Message{Status: c.status.String(), MsgType: IpcInternal}
			c.incoming <- &Message{Err: errors.New("timed out trying to re-connect"), MsgType: IpcInternal}
		}

		return
	}

	c.status = Connected
	c.incoming <- &Message{Status: c.status.String(), MsgType: IpcInternal}

	go c.clientReadDataFromConnectionToIncomingChannel()
}

// Read - blocking function that receives messages
// if MsgType is a negative number it's an internal message
func (c *Client) Read() (*Message, error) {
	m, ok := <-c.incoming
	if !ok {
		return nil, errors.New("client the received channel has been closed")
	}

	if m.Err != nil {
		close(c.incoming)
		close(c.outgoing)

		return nil, m.Err
	}

	return m, nil
}

// Write - writes a  message to the ipc connection.
// msgType - denotes the type of data being sent. 0 is a reserved type for internal messages and errors.
func (c *Client) Write(msgType MsgType, message []byte) error {
	if msgType == IpcCtrl {
		return errors.New(fmt.Sprintf("client message type %d is reserved", IpcCtrl))
	}

	if c.status != Connected {
		return errors.New(c.status.String())
	}

	mlen := len(message)
	if mlen > c.conf.MaxMsgSize {
		return errors.New("client message exceeds maximum message length")
	}

	c.outgoing <- &Message{MsgType: msgType, Data: message}

	return nil
}

// clientWriteDataFromOutgoingChannelToConnection a message to Client.outgoing channel
// eventually a message is structured as follows: lengthOfMsgTypePlusMessage + MsgType + Message
func (c *Client) clientWriteDataFromOutgoingChannelToConnection() {
	var err error
	for {
		msg, ok := <-c.outgoing
		if !ok {
			break
		}

		// eventually sending: MsgType + Message
		toSend := msg.MsgType.toBytes()
		toSend = append(toSend, msg.Data...)

		if c.conf.Encryption {
			toSend, err = encrypt(*c.enc.cipher, toSend)
			if err != nil {
				log.Debugln("client error encrypting data", err)
				continue
			}
		}

		writer := bufio.NewWriter(c.conn)
		writer.Write(intToBytes(len(toSend)))
		writer.Write(toSend)
		err = writer.Flush()
		if err != nil {
			log.Debugln("client error flushing data", err)
			continue
		}
	}
}

// Status StatusCode - returns the current connection status
func (c *Client) Status() Status {
	return c.status
}

// Close - closes the connection
func (c *Client) Close() {
	c.status = Closing
	if c.conn != nil {
		c.conn.Close()
	}
}
