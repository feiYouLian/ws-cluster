package wire

import (
	"io"
)

const (
	// MsgStateUnsend  未发送
	MsgStateUnsend = iota
	// MsgStateSent 1：发送成功
	MsgStateSent
	// MsgStateRead 2：已读
	MsgStateRead
)

// MsgchatAck 单聊消息应答
type MsgchatAck struct {
	header *MessageHeader
	State  uint8 // 0:未发送 1：已发送 2：已读
}

// decode Decode
func (m *MsgchatAck) decode(r io.Reader) error {
	var err error
	if m.State, err = ReadUint8(r); err != nil {
		return err
	}
	return nil
}

// encode Encode
func (m *MsgchatAck) encode(w io.Writer) error {
	var err error
	if err = WriteUint8(w, m.State); err != nil {
		return err
	}
	return nil
}

// Header 头信息
func (m *MsgchatAck) Header() *MessageHeader {
	return &MessageHeader{m.header.ID, MsgTypeChatAck, ScopeChat, m.header.To}
}
