// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"encoding/binary"
	"io"
)

var (
	// littleEndian is a convenience variable since binary.LittleEndian is
	// quite long.
	littleEndian = binary.LittleEndian
)

// ReadInt8 从 reader 中读取一个 int8
func ReadInt8(r io.Reader) (int8, error) {
	var bytes = make([]byte, 1)
	if _, err := io.ReadFull(r, bytes); err != nil {
		return 0, err
	}
	return int8(bytes[0]), nil
}

// ReadInt32 从 reader 中读取一个 int32
func ReadInt32(r io.Reader) (int32, error) {
	var bytes = make([]byte, 4)
	if _, err := io.ReadFull(r, bytes); err != nil {
		return 0, err
	}
	return int32(littleEndian.Uint32(bytes)), nil
}

// ReadInt64 从 reader 中读取一个 int64
func ReadInt64(r io.Reader) (int64, error) {
	var bytes = make([]byte, 8)
	if _, err := io.ReadFull(r, bytes); err != nil {
		return 0, err
	}
	return int64(littleEndian.Uint64(bytes)), nil
}

// ReadString 从 reader 中读取一个 string
func ReadString(r io.Reader) (string, error) {
	len, err := ReadInt32(r)
	if err != nil {
		return "", err
	}
	buf := make([]byte, len)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// WriteInt8 写一个 int8到 writer 中
func WriteInt8(w io.Writer, val int8) error {
	buf := []byte{byte(val)}
	if _, err := w.Write(buf); err != nil {
		return err
	}
	return nil
}

// WriteInt32 写一个 int32到 writer 中
func WriteInt32(w io.Writer, val int32) error {
	buf := make([]byte, 4)
	littleEndian.PutUint32(buf, uint32(val))
	if _, err := w.Write(buf); err != nil {
		return err
	}
	return nil
}

// WriteInt64 写一个 int64到 writer 中
func WriteInt64(w io.Writer, val int64) error {
	buf := make([]byte, 8)
	littleEndian.PutUint64(buf, uint64(val))
	if _, err := w.Write(buf); err != nil {
		return err
	}
	return nil
}

// WriteString 写一个 string 到 writer 中
func WriteString(w io.Writer, str string) error {
	slen := len(str)

	if err := WriteInt32(w, int32(slen)); err != nil {
		return err
	}

	if _, err := w.Write([]byte(str)); err != nil {
		return err
	}
	return nil
}
