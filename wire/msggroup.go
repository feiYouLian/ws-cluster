package wire

import "io"

// Msggroup 群组消息
type Msggroup struct {
	ID    uint64
	From  uint64
	Group string
	Text  string
}

// Decode Decode
func (m *Msggroup) Decode(r io.Reader) error {
	var err error
	if m.ID, err = ReadUint64(r); err != nil {
		return err
	}
	if m.From, err = ReadUint64(r); err != nil {
		return err
	}

	if m.Group, err = ReadString(r); err != nil {
		return err
	}
	if m.Text, err = ReadString(r); err != nil {
		return err
	}

	return nil
}

// Encode Encode
func (m *Msggroup) Encode(w io.Writer) error {
	var err error
	if err = WriteUint64(w, m.ID); err != nil {
		return err
	}
	if err = WriteUint64(w, m.From); err != nil {
		return err
	}
	if err = WriteString(w, m.Group); err != nil {
		return err
	}
	if err = WriteString(w, m.Text); err != nil {
		return err
	}
	return nil
}

// Msgtype 头信息
func (m *Msggroup) Msgtype() uint8 {
	return MsgTypeGroup
}
