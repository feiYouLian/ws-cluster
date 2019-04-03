package wire

import (
	"io"
)

// MsgKill 通知连接下线
type MsgKill struct {
	header *MessageHeader
}

// decode Decode
func (m *MsgKill) decode(r io.Reader) error {
	return nil
}

// encode Encode
func (m *MsgKill) encode(w io.Writer) error {
	return nil
}

// Header 头信息
func (m *MsgKill) Header() *MessageHeader {
	return &MessageHeader{m.header.ID, MsgTypeKill, ScopeClient, m.header.To}
}
