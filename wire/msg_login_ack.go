package wire

import (
	"io"
)

// MsgLoginAck 单聊消息应答
type MsgLoginAck struct {
	RemoteAddr string
	LoginAt    uint64
}

// Decode Decode
func (m *MsgLoginAck) Decode(r io.Reader) error {
	var err error
	if m.RemoteAddr, err = ReadString(r); err != nil {
		return err
	}
	if m.LoginAt, err = ReadUint64(r); err != nil {
		return err
	}
	return nil
}

// Encode Encode
func (m *MsgLoginAck) Encode(w io.Writer) error {
	var err error
	if err = WriteString(w, m.RemoteAddr); err != nil {
		return err
	}
	if err = WriteUint64(w, m.LoginAt); err != nil {
		return err
	}
	return nil
}
