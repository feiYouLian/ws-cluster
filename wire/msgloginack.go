package wire

import (
	"io"
)

// MsgLoginAck 单聊消息应答
type MsgLoginAck struct {
	header *MessageHeader
	PeerID string
}

// decode Decode
func (m *MsgLoginAck) decode(r io.Reader) error {
	var err error
	if m.PeerID, err = ReadString(r); err != nil {
		return err
	}
	return nil
}

// encode Encode
func (m *MsgLoginAck) encode(w io.Writer) error {
	var err error
	if err = WriteString(w, m.PeerID); err != nil {
		return err
	}
	return nil
}

// Header 头信息
func (m *MsgLoginAck) Header() *MessageHeader {
	return &MessageHeader{m.header.ID, MsgTypeLoginAck, ScopeNull, ""}
}
