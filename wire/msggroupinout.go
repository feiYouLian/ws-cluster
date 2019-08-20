package wire

import (
	"io"
	"strings"
)

const (
	// GroupIn join group
	GroupIn = 1
	// GroupOut leave group
	GroupOut = 0
)

// MsgGroupInOut 进入、退出 群
type MsgGroupInOut struct {
	InOut  uint8 //1 in 0: out
	Groups []string
}

// Decode Decode
func (m *MsgGroupInOut) Decode(r io.Reader) error {
	var err error
	if m.InOut, err = ReadUint8(r); err != nil {
		return err
	}
	grouparr, err := ReadString(r)
	if err != nil {
		return err
	}
	m.Groups = strings.Split(grouparr, ",")
	return nil
}

// Encode Encode
func (m *MsgGroupInOut) Encode(w io.Writer) error {
	var err error
	if err = WriteUint8(w, m.InOut); err != nil {
		return err
	}
	if err = WriteString(w, strings.Join(m.Groups, ",")); err != nil {
		return err
	}
	return nil
}
