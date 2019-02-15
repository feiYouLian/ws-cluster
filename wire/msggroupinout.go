package wire

import (
	"io"
	"strings"
)

// MsgGroupInOut 进入、退出 群
type MsgGroupInOut struct {
	header *MessageHeader
	InOut  uint8 //1 in 0: out
	Groups []string
}

// decode Decode
func (m *MsgGroupInOut) decode(r io.Reader) error {
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

// encode Encode
func (m *MsgGroupInOut) encode(w io.Writer) error {
	var err error
	if err = WriteUint8(w, m.InOut); err != nil {
		return err
	}
	if err = WriteString(w, strings.Join(m.Groups, ",")); err != nil {
		return err
	}
	return nil
}

// Header 头信息
func (m *MsgGroupInOut) Header() *MessageHeader {
	return &MessageHeader{m.header.ID, MsgTypeJoinGroup, 0, ""}
}
