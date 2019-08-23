package wire

import (
	"io"
)

const (
	// AckSucc server success done it
	AckSucc = uint8(1)
	// AckFail server fail done it
	AckFail = uint8(2)
	// AckRead dest peer has read it
	AckRead = uint8(3)
)

// MsgChatResp 单聊消息应答
type MsgChatResp struct {
	State uint8
	Err   string
}

// Decode Decode
func (m *MsgChatResp) Decode(r io.Reader) error {
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
func (m *MsgChatResp) Encode(w io.Writer) error {
	var err error
	if err = WriteUint8(w, m.State); err != nil {
		return err
	}
	if err = WriteString(w, m.Err); err != nil {
		return err
	}
	return nil
}
