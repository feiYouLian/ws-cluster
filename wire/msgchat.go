package wire

import (
	"io"
)

// Msgchat 聊天消息
type Msgchat struct {
	Type  uint8 // 1: text 2: image
	Text  string
	Extra string
}

// Decode Decode
func (m *Msgchat) Decode(r io.Reader) error {
	var err error
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

// Encode Encode
func (m *Msgchat) Encode(w io.Writer) error {
	var err error
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
