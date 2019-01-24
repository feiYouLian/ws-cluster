package wire

import "io"

// Msgchat 单聊消息
type Msgchat struct {
	ID   int64
	From int64
	To   int64
	Type int8
	Text string
}

// Decode Decode
func (m *Msgchat) Decode(r io.Reader) error {
	var err error
	if m.ID, err = ReadInt64(r); err != nil {
		return err
	}
	if m.From, err = ReadInt64(r); err != nil {
		return err
	}
	if m.To, err = ReadInt64(r); err != nil {
		return err
	}
	if m.Type, err = ReadInt8(r); err != nil {
		return err
	}
	if m.Text, err = ReadString(r); err != nil {
		return err
	}

	return nil
}

// Encode Encode
func (m *Msgchat) Encode(w io.Writer) error {
	var err error
	if err = WriteInt64(w, m.ID); err != nil {
		return err
	}
	if err = WriteInt64(w, m.From); err != nil {
		return err
	}
	if err = WriteInt64(w, m.To); err != nil {
		return err
	}
	if err = WriteInt8(w, m.Type); err != nil {
		return err
	}
	if err = WriteString(w, m.Text); err != nil {
		return err
	}
	return nil
}

// Header 头信息
func (m *Msgchat) Header() int32 {
	return makeHeader(MsgTypeChat)
}
