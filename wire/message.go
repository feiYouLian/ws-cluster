package wire

import (
	"fmt"
	"io"
)

const (
	// MsgTypeLoginAck login ack
	MsgTypeLoginAck = uint8(1)
	// MsgTypeChat 单聊消息
	MsgTypeChat = uint8(3)
	// MsgTypeChatResp 应答消息
	MsgTypeChatResp = uint8(4)
	// MsgTypeGroupInOut join or leave group
	MsgTypeGroupInOut = uint8(5)
	// MsgTypeKill kill a client
	MsgTypeKill = uint8(7)
	// MsgTypeLoc locate message
	MsgTypeLoc = uint8(9)
	// MsgTypeOffline offline notice message
	MsgTypeOffline = uint8(11)
	// MsgTypeOfflineNotice MsgTypeOfflineNotice
	MsgTypeOfflineNotice = uint8(13)

	// MsgTypeEmpty MsgTypeEmpty
	MsgTypeEmpty = uint8(200)
)

// const (
// 	// ScopeNull no target
// 	ScopeNull = uint8(0)
// 	// ScopeClient msg to client
// 	ScopeClient = uint8(1)
// 	// ScopeGroup msg to a group
// 	ScopeGroup = uint8(3)
// )

// Protocol defined message decode and encode function
type Protocol interface {
	Decode(io.Reader) error
	Encode(io.Writer) error
}

// Header is Message Header
type Header struct {
	Source  Addr   //source address
	Dest    Addr   //destination address
	Seq     uint32 //消息序列号，peer唯一
	AckSeq  uint32 //应答消息序列号
	Command uint8  //命令类型
	Status  uint8  // respose status
}

// Decode Decode reader to Header
func (h *Header) Decode(r io.Reader) error {
	var err error
	if err = h.Source.Decode(r); err != nil {
		return err
	}
	if err = h.Dest.Decode(r); err != nil {
		return err
	}
	if h.Seq, err = ReadUint32(r); err != nil {
		return err
	}
	if h.AckSeq, err = ReadUint32(r); err != nil {
		return err
	}
	if h.Command, err = ReadUint8(r); err != nil {
		return err
	}
	if h.Status, err = ReadUint8(r); err != nil {
		return err
	}
	return nil
}

// Encode Encode Header to writer
func (h *Header) Encode(w io.Writer) error {
	var err error
	if err = h.Source.Encode(w); err != nil {
		return err
	}
	if err = h.Dest.Encode(w); err != nil {
		return err
	}
	if err = WriteUint32(w, h.Seq); err != nil {
		return err
	}
	if err = WriteUint32(w, h.AckSeq); err != nil {
		return err
	}
	if err = WriteUint8(w, h.Command); err != nil {
		return err
	}
	if err = WriteUint8(w, h.Status); err != nil {
		return err
	}
	return nil
}

func (h *Header) String() string {
	return fmt.Sprintf("Header[Seq:%v; %v->%v; Ack:%v Command:%v]", h.Seq, h.Source.String(), h.Dest.String(), h.AckSeq, h.Command)
}

// Message Message
type Message struct {
	Header *Header
	Body   Protocol
}

// Decode Decode reader to Message
func (m *Message) Decode(r io.Reader) error {
	var err error
	m.Header = &Header{}
	if err := m.Header.Decode(r); err != nil {
		return err
	}
	m.Body, err = MakeEmptyBody(m.Header.Command)
	if err != nil {
		return err
	}
	if err = m.Body.Decode(r); err != nil {
		return err
	}
	return nil
}

// Encode Encode Header to Message
func (m *Message) Encode(w io.Writer) error {
	if err := m.Header.Encode(w); err != nil {
		return err
	}
	if err := m.Body.Encode(w); err != nil {
		return err
	}
	return nil
}

// MakeEmptyBody 创建一个空的消息体
func MakeEmptyBody(Command uint8) (Protocol, error) {
	var body Protocol
	switch Command {
	case MsgTypeChat:
		body = &Msgchat{}
	case MsgTypeChatResp:
		body = &MsgChatResp{}
	case MsgTypeGroupInOut:
		body = &MsgGroupInOut{}
	case MsgTypeKill:
		body = &MsgKill{}
	case MsgTypeLoginAck:
		body = &MsgLoginAck{}
	case MsgTypeLoc:
		body = &MsgLoc{}
	case MsgTypeOffline:
		body = &MsgOffline{}
	case MsgTypeOfflineNotice:
		body = &MsgOfflineNotice{}
	case MsgTypeEmpty:
		body = &MsgEmpty{}
	default:
		return nil, fmt.Errorf("unhandled msgType[%d]", Command)
	}
	return body, nil
}

// MakeEmptyHeaderMessage Make a Message which header is empty
func MakeEmptyHeaderMessage(Command uint8, body Protocol) *Message {
	return &Message{
		Header: &Header{
			Command: Command,
		},
		Body: body,
	}
}

// MakeEmptyRespMessage Make a Message which body is MsgEmpty
func MakeEmptyRespMessage(from *Header, status uint8) *Message {
	header := &Header{
		Source:  from.Dest,
		Dest:    from.Source,
		AckSeq:  from.Seq,
		Command: MsgTypeEmpty,
		Status:  status,
	}
	return &Message{
		Header: header,
		Body:   &MsgEmpty{},
	}
}
