package wire

import "io"

const (
	// MsgStateUnsend  未发送
	MsgStateUnsend = iota
	// MsgStateSent 1：已发送
	MsgStateSent
	// MsgStateRead 2：已读
	MsgStateRead
)

// MsgchatAck 单聊消息应答
type MsgchatAck struct {
	ID    uint64
	To    uint64
	State uint8 // 0:未发送 1：已发送 2：已读
	Text  string
}

// decode Decode
func (m *MsgchatAck) decode(r io.Reader) error {
	var err error
	if m.ID, err = ReadUint64(r); err != nil {
		return err
	}
	if m.To, err = ReadUint64(r); err != nil {
		return err
	}
	if m.State, err = ReadUint8(r); err != nil {
		return err
	}
	if m.Text, err = ReadString(r); err != nil {
		return err
	}

	return nil
}

// encode Encode
func (m *MsgchatAck) encode(w io.Writer) error {
	var err error
	if err = WriteUint64(w, m.ID); err != nil {
		return err
	}
	if err = WriteUint64(w, m.To); err != nil {
		return err
	}
	if err = WriteUint8(w, m.State); err != nil {
		return err
	}
	if err = WriteString(w, m.Text); err != nil {
		return err
	}
	return nil
}

// Msgtype 头信息
func (m *MsgchatAck) Msgtype() uint8 {
	return MsgTypeChat
}
