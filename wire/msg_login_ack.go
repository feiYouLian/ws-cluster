package wire

import (
	"io"
)

// MsgLoginAck 单聊消息应答
type MsgLoginAck struct {
	PeerID string
}

// Decode Decode
func (m *MsgLoginAck) Decode(r io.Reader) error {
	var err error
	if m.PeerID, err = ReadString(r); err != nil {
		return err
	}
	return nil
}

// Encode Encode
func (m *MsgLoginAck) Encode(w io.Writer) error {
	var err error
	if err = WriteString(w, m.PeerID); err != nil {
		return err
	}
	return nil
}
