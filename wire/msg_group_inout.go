package wire

import (
	"io"
)

const (
	// GroupIn join group
	GroupIn = 1
	// GroupOut leave group
	GroupOut = 0
)

// MsgGroupInOut 进入、退出 群
type MsgGroupInOut struct {
	InOut  uint8  //1 in 0: out
	Groups []Addr // max number is 127
}

// Decode Decode
func (m *MsgGroupInOut) Decode(r io.Reader) error {
	var err error
	if m.InOut, err = ReadUint8(r); err != nil {
		return err
	}
	num, _ := ReadUint8(r)

	m.Groups = make([]Addr, 0)
	for i := uint8(0); i < num; i++ {
		addr := new(Addr)
		if err := addr.Decode(r); err != nil {
			return err
		}
		m.Groups = append(m.Groups, *addr)
	}
	return nil
}

// Encode Encode
func (m *MsgGroupInOut) Encode(w io.Writer) error {
	var err error
	if err = WriteUint8(w, m.InOut); err != nil {
		return err
	}
	if err = WriteUint8(w, uint8(len(m.Groups))); err != nil {
		return err
	}
	for _, addr := range m.Groups {
		if err := addr.Encode(w); err != nil {
			return err
		}
	}
	return nil
}
