package wire

import (
	"io"
)

// MsgQueryClientResp location message
type MsgQueryClientResp struct {
	LoginAt uint32 // 0 not online
}

// Decode Decode
func (m *MsgQueryClientResp) Decode(r io.Reader) error {
	var err error
	if m.LoginAt, err = ReadUint32(r); err != nil {
		return err
	}
	return nil
}

// Encode Encode
func (m *MsgQueryClientResp) Encode(w io.Writer) error {
	var err error
	if err = WriteUint32(w, m.LoginAt); err != nil {
		return err
	}
	return nil
}
