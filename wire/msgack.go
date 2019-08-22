package wire

import (
	"io"
)

const (
	// AckDone server has deal with it
	AckDone = uint8(1)
	// AckRead dest peer has read it
	AckRead = uint8(3)
)

// MsgAck 单聊消息应答
type MsgAck struct {
	State uint8
	Err   string
}

// Decode Decode
func (m *MsgAck) Decode(r io.Reader) error {
	var err error
	if m.State, err = ReadUint8(r); err != nil {
		return err
	}
	if m.Err, err = ReadString(r); err != nil {
		return err
	}
	return nil
}

// Encode Encode
func (m *MsgAck) Encode(w io.Writer) error {
	var err error
	if err = WriteUint8(w, m.State); err != nil {
		return err
	}
	if err = WriteString(w, m.Err); err != nil {
		return err
	}
	return nil
}
