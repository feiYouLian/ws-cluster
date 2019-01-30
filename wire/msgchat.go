package wire

import "io"

const (
	// ChatTypeSingle 单聊
	ChatTypeSingle = 1
	// ChatTypeGroup 群聊
	ChatTypeGroup = 2
)

// Msgchat 单聊消息
type Msgchat struct {
	ID   uint64
	From uint64
	To   uint64
	Type uint8 // 1: text 2: image
	Text string
}

// decode decode
func (m *Msgchat) decode(r io.Reader) error {
	var err error
	if m.ID, err = ReadUint64(r); err != nil {
		return err
	}
	if m.From, err = ReadUint64(r); err != nil {
		return err
	}
	if m.To, err = ReadUint64(r); err != nil {
		return err
	}
	if m.Type, err = ReadUint8(r); err != nil {
		return err
	}
	if m.Text, err = ReadString(r); err != nil {
		return err
	}

	return nil
}

// encode encode
func (m *Msgchat) encode(w io.Writer) error {
	var err error
	if err = WriteUint64(w, m.ID); err != nil {
		return err
	}
	if err = WriteUint64(w, m.From); err != nil {
		return err
	}
	if err = WriteUint64(w, m.To); err != nil {
		return err
	}
	if err = WriteUint8(w, m.Type); err != nil {
		return err
	}
	if err = WriteString(w, m.Text); err != nil {
		return err
	}
	return nil
}

// Msgtype 头信息
func (m *Msgchat) Msgtype() uint8 {
	return MsgTypeChat
}
