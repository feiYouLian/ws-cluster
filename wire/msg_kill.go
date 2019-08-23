package wire

import "io"

// MsgKill 通知连接下线
type MsgKill struct {
	LoginAt uint64
}

// Decode Decode
func (m *MsgKill) Decode(r io.Reader) error {
	var err error
	if m.LoginAt, err = ReadUint64(r); err != nil {
		return err
	}
	return nil
}

// Encode Encode
func (m *MsgKill) Encode(w io.Writer) error {
	var err error
	if err = WriteUint64(w, m.LoginAt); err != nil {
		return err
	}
	return nil
}
