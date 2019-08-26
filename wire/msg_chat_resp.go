package wire

import (
	"io"
)

const (
	// AckSent sent to client
	AckSent = uint8(1)
	// AckRead dest peer has read it
	AckRead = uint8(2)
	// AckFail server fail done it
	AckFail = uint8(9)
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
