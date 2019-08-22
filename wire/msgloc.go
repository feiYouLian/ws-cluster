package wire

import "io"

// MsgLoc location message
type MsgLoc struct {
	Target Addr // target address
	Server string
}

// Decode Decode
func (m *MsgLoc) Decode(r io.Reader) error {
	var err error
	if m.Target.Decode(r); err != nil {
		return err
	}
	if m.Server, err = ReadString(r); err != nil {
		return err
	}
	return nil
}

// Encode Encode
func (m *MsgLoc) Encode(w io.Writer) error {
	var err error
	if m.Target.Encode(w); err != nil {
		return err
	}
	if err = WriteString(w, m.Server); err != nil {
		return err
	}
	return nil
}
