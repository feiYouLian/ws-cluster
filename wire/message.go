package wire

import (
	"fmt"
	"io"
)

const (

	// MsgTypeChat 单聊消息
	MsgTypeChat = uint8(3)
	// MsgTypeGroupInOut join or leave group
	MsgTypeGroupInOut = uint8(5)
	// MsgTypeKill kill a client
	MsgTypeKill = uint8(7)
	// MsgTypeLoginAck login ack
	MsgTypeLoginAck = uint8(100)
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
	return nil
}

// Message Message
type Message struct {
	Header *Header
	Body   Protocol
}

// Decode Decode reader to Message
func (m *Message) Decode(r io.Reader) error {
	if err := m.Header.Decode(r); err != nil {
		return err
	}
	if err := m.Body.Decode(r); err != nil {
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

// // ReadHeader read header
// func ReadHeader(r io.Reader) (*MessageHeader, error) {
// 	header := &MessageHeader{}
// 	var err error
// 	// if header.ID, err = ReadUint32(r); err != nil {
// 	// 	return nil, err
// 	// }
// 	// if header.Msgtype, err = ReadUint8(r); err != nil {
// 	// 	return nil, err
// 	// }
// 	// if header.Scope, err = ReadUint8(r); err != nil {
// 	// 	return nil, err
// 	// }
// 	// if header.Scope == ScopeNull {
// 	// 	return header, nil
// 	// }
// 	// if header.To, err = ReadString(r); err != nil {
// 	// 	return nil, err
// 	// }
// 	return header, nil
// }

// // WriteHeader write header to writer
// func WriteHeader(w io.Writer, msg Message) error {
// 	header := msg.Header()
// 	if err := WriteUint32(w, header.ID); err != nil {
// 		return err
// 	}
// 	// if err := WriteUint8(w, header.Msgtype); err != nil {
// 	// 	return err
// 	// }
// 	// if err := WriteUint8(w, header.Scope); err != nil {
// 	// 	return err
// 	// }
// 	// if header.Scope == ScopeNull {
// 	// 	return nil
// 	// }
// 	// if err := WriteString(w, header.To); err != nil {
// 	// 	return err
// 	// }
// 	return nil
// }

// // ReadMessage 从reader 中读取消息
// func ReadMessage(r io.Reader, Command uint8) (Protocol, error) {
// 	msg, err := MakeEmptyMessage(Command)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if err = msg.decode(r); err != nil {
// 		return nil, err
// 	}
// 	return msg, nil
// }

// // WriteMessage 把 msg 写到 w 中
// func WriteMessage(w io.Writer, msg Protocol) error {
// 	if err := msg.encode(w); err != nil {
// 		return err
// 	}
// 	return nil
// }

// MakeEmptyMessage 创建一个空的消息体
func MakeEmptyMessage(Command uint8) (*Message, error) {
	var msg Protocol
	switch Command {
	case MsgTypeChat:
		msg = &Msgchat{}
	case MsgTypeGroupInOut:
		msg = &MsgGroupInOut{}
	case MsgTypeKill:
		msg = &MsgKill{}
	case MsgTypeLoginAck:
		msg = &MsgLoginAck{}
	default:
		return nil, fmt.Errorf("unhandled msgType[%d]", Command)
	}
	return &Message{
		Header: &Header{
			Command: Command,
		},
		Body: msg,
	}, nil
}

// // MakeAckMessage make a ChatAck message
// func MakeAckMessage(id uint32, state uint8) ([]byte, error) {
// 	ackHeader := &MessageHeader{ID: id, Command: MsgTypeAck}
// 	ackMessage, _ := MakeEmptyMessage(ackHeader)
// 	msgAck, _ := ackMessage.(*MsgAck)
// 	// set state sent
// 	msgAck.State = state

// 	buf := &bytes.Buffer{}
// 	err := WriteMessage(buf, msgAck)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return buf.Bytes(), nil
// }

// // MakeKillMessage kill to
// func MakeKillMessage(id uint32, client string, peerID string) ([]byte, error) {
// 	header := &MessageHeader{ID: id, Command: MsgTypeKill}
// 	message, _ := MakeEmptyMessage(header)
// 	msgKill := message.(*MsgKill)
// 	msgKill.PeerID = peerID

// 	buf := &bytes.Buffer{}
// 	err := WriteMessage(buf, message)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return buf.Bytes(), nil
// }

// // MakeLoginAckMessage login ack
// func MakeLoginAckMessage(id uint32, peerID string) ([]byte, error) {
// 	header := &MessageHeader{ID: id, Command: MsgTypeLoginAck}
// 	message, _ := MakeEmptyMessage(header)
// 	msgLoginAck := message.(*MsgLoginAck)
// 	msgLoginAck.PeerID = peerID

// 	buf := &bytes.Buffer{}
// 	err := WriteMessage(buf, message)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return buf.Bytes(), nil
// }
