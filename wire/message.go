package wire

import (
	"fmt"
	"io"
)

const (
	// MsgTypeChat 单聊消息
	MsgTypeChat = uint8(3)
	// MsgTypeGroup 群消息
	MsgTypeGroup = uint8(5)
)

// Message 定义了消息接口，消息必须有序列化和反序列化方法
type Message interface {
	Decode(io.Reader) error
	Encode(io.Writer) error
	// Header 前16位是消息类型，后16位保留
	Header() int32
}

func makeHeader(msgType uint8) int32 {
	return int32(msgType) << 24
}

// ReadMessage 从reader 中读取消息
func ReadMessage(r io.Reader) (Message, error) {
	header, err := ReadInt32(r)
	if err != nil {
		return nil, err
	}
	msg, err := makeEmptyMessage(header)
	if err != nil {
		return nil, err
	}
	if err = msg.Decode(r); err != nil {
		return nil, err
	}
	return msg, nil
}

// WriteMessage 把 msg 写到 w 中
func WriteMessage(w io.Writer, msg Message) error {
	if err := WriteInt32(w, msg.Header()); err != nil {
		return err
	}
	if err := msg.Encode(w); err != nil {
		return err
	}
	return nil
}

func makeEmptyMessage(header int32) (Message, error) {
	msgType := header >> 24
	var msg Message
	switch uint8(msgType) {
	case MsgTypeChat:
		msg = &Msgchat{}
	default:
		return nil, fmt.Errorf("unhandled header[%d]", header)
	}

	return msg, nil
}
