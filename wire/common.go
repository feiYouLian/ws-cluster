// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
)

var (
	// littleEndian is a convenience variable since binary.LittleEndian is
	// quite long.
	littleEndian = binary.LittleEndian
)

const (
	// AddrVirtual AddrVirtual
	AddrVirtual = byte(0)
	// AddrPeer peer address
	AddrPeer = byte(1)
	// AddrServer is server addr
	AddrServer = byte(2)
	// AddrGroup group address
	AddrGroup = byte(3)
	// AddrBroadcast broadcast address
	AddrBroadcast = byte(4)
)

// AddrMap addr byte to char
var AddrMap = map[byte]rune{
	AddrPeer:      'p',
	AddrServer:    's',
	AddrGroup:     'g',
	AddrBroadcast: 'b',
}

// AddrReMap AddrReMap
var AddrReMap = map[string]byte{
	"p": AddrPeer,
	"s": AddrServer,
	"g": AddrGroup,
	"b": AddrBroadcast,
}

var (
	// ErrAddrOverflow ErrAddrOverflow
	ErrAddrOverflow = errors.New("address is overflow")
	// ErrInvaildAddress ErrInvaildAddress
	ErrInvaildAddress = errors.New("address is invaild")
)

var (
	// DeviceNone DeviceNone
	DeviceNone = byte(0) //none
	// DevicePhone phone
	DevicePhone = byte(1) //phone
	// DevicePad pad
	DevicePad = byte(2) //pad
	// DevicePc pc
	DevicePc = byte(3) // pc
)

// Addr Address
// /  3 bit     / 5 bit   / 4 byte  / 1byte / 26 byte /
// / message type/ length   / domain / device /address /
type Addr [32]byte

// NewAddr new an Addr object
func NewAddr(Typ byte, domain uint32, device byte, address string) (*Addr, error) {
	addr := new(Addr)
	addrBytes := []byte(address)
	addrlen := len(addrBytes)
	if addrlen > 26 {
		return nil, ErrAddrOverflow
	}
	addr[0] = byte(Typ<<5) | byte(addrlen)
	bs := make([]byte, 4)
	littleEndian.PutUint32(bs, domain)
	copy(addr[1:5], bs)
	addr[5] = device
	copy(addr[6:], addrBytes)

	return addr, nil
}

// NewGroupAddr new a group address
func NewGroupAddr(domain uint32, address string) (*Addr, error) {
	return NewAddr(AddrGroup, domain, DeviceNone, address)
}

// NewServerAddr new a server address
func NewServerAddr(domain uint32, address string) (*Addr, error) {
	return NewAddr(AddrServer, domain, DeviceNone, address)
}

// ParsePeerAddr new a peer address
func ParsePeerAddr(addr string) (*Addr, error) {
	addrs := strings.SplitN(addr, "/", 5)
	if len(addrs) != 5 {
		return nil, ErrInvaildAddress
	}
	domain, _ := strconv.Atoi(addrs[2])
	device, _ := strconv.Atoi(addrs[3])
	return NewAddr(AddrPeer, uint32(domain), byte(device), addrs[4])
}

// ParseAddr ParseAddr
func ParseAddr(addr string) (*Addr, error) {
	addrs := strings.SplitN(addr, "/", 5)
	if len(addrs) != 5 {
		return nil, ErrInvaildAddress
	}
	domain, _ := strconv.Atoi(addrs[2])
	device, _ := strconv.Atoi(addrs[3])
	return NewAddr(AddrReMap[addrs[1]], uint32(domain), byte(device), addrs[4])
}

// ParseGroupAddr new a group address
func ParseGroupAddr(addr string) (*Addr, error) {
	addrs := strings.SplitN(addr, "/", 5)
	if len(addrs) != 5 {
		return nil, ErrInvaildAddress
	}
	domain, _ := strconv.Atoi(addrs[2])
	device, _ := strconv.Atoi(addrs[3])
	return NewAddr(AddrGroup, uint32(domain), byte(device), addrs[4])
}

// ParseServerAddr new a server address
func ParseServerAddr(addr string) (*Addr, error) {
	addrs := strings.SplitN(addr, "/", 5)
	if len(addrs) != 5 {
		return nil, ErrInvaildAddress
	}
	domain, _ := strconv.Atoi(addrs[2])
	device, _ := strconv.Atoi(addrs[3])
	return NewAddr(AddrServer, uint32(domain), byte(device), addrs[4])
}

// Decode Decode reader to Header
func (addr *Addr) Decode(r io.Reader) error {
	_, err := r.Read(addr[0:])
	return err
}

// Encode Encode Header to writer
func (addr *Addr) Encode(w io.Writer) error {
	_, err := w.Write(addr[0:])
	return err
}

// Type address type,return AddrSingle ,AddrGroup ,AddrBroadcast
func (addr *Addr) Type() byte {
	return addr[0] >> 5
}

// Len address length
func (addr *Addr) Len() byte {
	return addr[0] & ^byte(3<<5)
}

// Domain domain is the scope of client
func (addr *Addr) Domain() uint32 {
	return littleEndian.Uint32(addr[1:5])
}

// Device Device
func (addr *Addr) Device() byte {
	return addr[5]
}

// Address address detail
func (addr *Addr) Address() string {
	return string(addr[6 : addr.Len()+6])
}

// Full full address
func (addr *Addr) String() string {
	if addr.Type() == AddrBroadcast {
		return fmt.Sprintf("/%c/%v", AddrMap[addr.Type()], addr.Domain())
	}
	return fmt.Sprintf("/%c/%v/%v/%v", AddrMap[addr.Type()], addr.Domain(), addr.Device(), addr.Address())
}

// IsEmpty address is empty
func (addr *Addr) IsEmpty() bool {
	return addr[0] == 0
}

// ReadUint8 从 reader 中读取一个 uint8
func ReadUint8(r io.Reader) (uint8, error) {
	var bytes = make([]byte, 1)
	if _, err := io.ReadFull(r, bytes); err != nil {
		return 0, err
	}
	return uint8(bytes[0]), nil
}

// ReadUint32 从 reader 中读取一个 uint32
func ReadUint32(r io.Reader) (uint32, error) {
	var bytes = make([]byte, 4)
	if _, err := io.ReadFull(r, bytes); err != nil {
		return 0, err
	}
	return littleEndian.Uint32(bytes), nil
}

// ReadUint16 从 reader 中读取一个 uint16
func ReadUint16(r io.Reader) (uint16, error) {
	var bytes = make([]byte, 2)
	if _, err := io.ReadFull(r, bytes); err != nil {
		return 0, err
	}
	return littleEndian.Uint16(bytes), nil
}

// ReadUint64 从 reader 中读取一个 uint64
func ReadUint64(r io.Reader) (uint64, error) {
	var bytes = make([]byte, 8)
	if _, err := io.ReadFull(r, bytes); err != nil {
		return 0, err
	}
	return littleEndian.Uint64(bytes), nil
}

// ReadString 从 reader 中读取一个 string
func ReadString(r io.Reader) (string, error) {
	buf, err := ReadBytes(r)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// ReadBytes 从 reader 中读取一个 []byte, reader中前4byte 必须是[]byte 的长度
func ReadBytes(r io.Reader) ([]byte, error) {
	len, err := ReadUint32(r)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, len)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// WriteUint8 写一个 uint8到 writer 中
func WriteUint8(w io.Writer, val uint8) error {
	buf := []byte{byte(val)}
	if _, err := w.Write(buf); err != nil {
		return err
	}
	return nil
}

// WriteUint16 写一个 int16到 writer 中
func WriteUint16(w io.Writer, val uint16) error {
	buf := make([]byte, 2)
	littleEndian.PutUint16(buf, val)
	if _, err := w.Write(buf); err != nil {
		return err
	}
	return nil
}

// WriteUint32 写一个 int32到 writer 中
func WriteUint32(w io.Writer, val uint32) error {
	buf := make([]byte, 4)
	littleEndian.PutUint32(buf, val)
	if _, err := w.Write(buf); err != nil {
		return err
	}
	return nil
}

// WriteUint64 写一个 int64到 writer 中
func WriteUint64(w io.Writer, val uint64) error {
	buf := make([]byte, 8)
	littleEndian.PutUint64(buf, val)
	if _, err := w.Write(buf); err != nil {
		return err
	}
	return nil
}

// WriteString 写一个 string 到 writer 中
func WriteString(w io.Writer, str string) error {
	if err := WriteBytes(w, []byte(str)); err != nil {
		return err
	}
	return nil
}

// WriteBytes 写一个 buf []byte 到 writer 中
func WriteBytes(w io.Writer, buf []byte) error {
	slen := len(buf)

	if err := WriteUint32(w, uint32(slen)); err != nil {
		return err
	}
	if _, err := w.Write(buf); err != nil {
		return err
	}
	return nil
}

// ErrIsNotAscll is not ascall error
var ErrIsNotAscll = errors.New("ErrIsNotAscll")

// WriteAscllString WriteAscllString
func WriteAscllString(w io.Writer, str string, capacity int) error {
	buf := make([]byte, capacity)
	for i, ch := range str {
		if ch > 127 {
			return ErrIsNotAscll
		}
		buf[i] = byte(ch)
	}
	log.Print(buf)
	_, err := w.Write(buf)
	return err
}

// ReadAscllString readAscllString
func ReadAscllString(r io.Reader, str string, capacity int) (string, error) {
	buf := make([]byte, capacity)
	_, err := r.Read(buf)
	if err != nil {
		return "", err
	}
	log.Print(buf)
	i := 0
	for ; i < len(buf); i++ {
		if buf[i] == 0 {
			break
		}
	}
	return string(buf[0:i]), err
}

// func (s64 *string64) String() string {
// 	for i := range s64 {
// 		if s64[i] == 0 {
// 			return string(s64[0:i])
// 		}
// 	}
// 	return ""
// }
