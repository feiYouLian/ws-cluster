package wire

import "io"

// MsgOffline client offline notice message
type MsgOffline struct {
	Peer    Addr   // target address
	Notice  uint8  // is notice to target  1:yes 0:no
	Targets []Addr // notice Target Peers

}

// Decode Decode
func (m *MsgOffline) Decode(r io.Reader) error {
	var err error
	if m.Peer.Decode(r); err != nil {
		return err
	}
	if m.Notice, err = ReadUint8(r); err != nil {
		return err
	}
	var num uint16
	if num, err = ReadUint16(r); err != nil {
		return err
	}
	m.Targets = make([]Addr, num)
	for i := uint16(0); i < num; i++ {
		addr := Addr{}
		if err := addr.Decode(r); err != nil {
			return err
		}
		m.Targets[i] = addr
	}
	return nil
}

// Encode Encode
func (m *MsgOffline) Encode(w io.Writer) error {
	var err error
	if m.Peer.Encode(w); err != nil {
		return err
	}
	if err = WriteUint8(w, m.Notice); err != nil {
		return err
	}
	if err = WriteUint16(w, uint16(len(m.Targets))); err != nil {
		return err
	}
	for _, addr := range m.Targets {
		if err := addr.Encode(w); err != nil {
			return err
		}
	}
	return nil
}
