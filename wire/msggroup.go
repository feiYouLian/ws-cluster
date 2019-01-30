package wire

import (
	"io"
)

// Msggroup 群组消息
type Msggroup struct {
	header *MessageHeader
	ID     uint64
	From   uint64
	Text   string
}

// decode Decode
func (m *Msggroup) decode(r io.Reader) error {
	var err error
	if m.ID, err = ReadUint64(r); err != nil {
		return err
	}
	if m.From, err = ReadUint64(r); err != nil {
		return err
	}
	if m.Text, err = ReadString(r); err != nil {
		return err
	}

	return nil
}

// encode Encode
func (m *Msggroup) encode(w io.Writer) error {
	var err error
	if err = WriteUint64(w, m.ID); err != nil {
		return err
	}
	if err = WriteUint64(w, m.From); err != nil {
		return err
	}
	if err = WriteString(w, m.Text); err != nil {
		return err
	}
	return nil
}

// Header 头信息
func (m *Msggroup) Header() *MessageHeader {
	return &MessageHeader{MsgTypeGroup, ScopeChat, m.header.To}
}
