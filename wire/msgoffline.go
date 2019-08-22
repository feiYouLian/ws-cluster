package wire

import "io"

// MsgOffline client offline notice message
type MsgOffline struct {
	Target Addr // target address
}

// Decode Decode
func (m *MsgOffline) Decode(r io.Reader) error {
	var err error
	if m.Target.Decode(r); err != nil {
		return err
	}
	return nil
}

// Encode Encode
func (m *MsgOffline) Encode(w io.Writer) error {
	var err error
	if m.Target.Encode(w); err != nil {
		return err
	}
	return nil
}
