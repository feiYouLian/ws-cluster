package wire

import (
	"io"
)

// MsgAck 单聊消息应答
type MsgAck struct {
	header *MessageHeader
	State  uint8 //
	Desc   string
}

// decode Decode
func (m *MsgAck) decode(r io.Reader) error {
	var err error
	if m.State, err = ReadUint8(r); err != nil {
		return err
	}
	if m.Desc, err = ReadString(r); err != nil {
		return err
	}
	return nil
}

// encode Encode
func (m *MsgAck) encode(w io.Writer) error {
	var err error
	if err = WriteUint8(w, m.State); err != nil {
		return err
	}
	if err = WriteString(w, m.Desc); err != nil {
		return err
	}
	return nil
}

// Header 头信息
func (m *MsgAck) Header() *MessageHeader {
	return &MessageHeader{m.header.ID, MsgTypeAck, ScopeChat, m.header.To}
}
