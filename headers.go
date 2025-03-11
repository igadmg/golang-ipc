package ipc

import (
	"bytes"
	"encoding/binary"
)

func (mt MsgType) toBytes() []byte {
	return intToBytes(int(mt))
}
func intToBytes(mLen int) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(mLen))
	return b
}

func bytesToMsgType(b []byte) MsgType {
	return MsgType(bytesToInt(b))
}
func bytesToInt(b []byte) int {
	var mlen uint32
	binary.Read(bytes.NewReader(b[:]), binary.BigEndian, &mlen) // message length
	return int(mlen)
}
