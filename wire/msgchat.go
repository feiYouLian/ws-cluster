package wire

import (
	"io"
)

// Msgchat 聊天消息
type Msgchat struct {
	header *MessageHeader
	From   string
	Type   uint8 // 1: text 2: image
	Text   string
	Extra  string
}

// decode decode
func (m *Msgchat) decode(r io.Reader) error {
	var err error
	if m.From, err = ReadString(r); err != nil {
		return err
	}
	if m.Type, err = ReadUint8(r); err != nil {
		return err
	}
	if m.Text, err = ReadString(r); err != nil {
		return err
	}
	if m.Extra, err = ReadString(r); err != nil {
		return err
	}

	return nil
}

// encode encode
func (m *Msgchat) encode(w io.Writer) error {
	var err error
	if err = WriteString(w, m.From); err != nil {
		return err
	}
	if err = WriteUint8(w, m.Type); err != nil {
		return err
	}
	if err = WriteString(w, m.Text); err != nil {
		return err
	}
	if err = WriteString(w, m.Extra); err != nil {
		return err
	}
	return nil
}

// Header 头信息
func (m *Msgchat) Header() *MessageHeader {
	return &MessageHeader{m.header.ID, MsgTypeChat, ScopeChat, m.header.To}
}
