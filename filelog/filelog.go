package filelog

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log"
	"os"
	"sync"
	"time"
)

const (
	blockSize = 4 * 1024
)

var (
	// littleEndian is a convenience variable since binary.LittleEndian is
	// quite long.
	littleEndian = binary.LittleEndian
)

var (
	errBlockLackOfSpace = errors.New("Lack of space")
	errBlockEmpty       = errors.New("block empty")
	errNoMoreBlock      = errors.New("no more block")
)

// Block log Block
type Block struct {
	buf    []byte
	offset uint16
	cap    uint16
	length uint16 //record count
}

// block 一个 blocke只能读或者写
func newBlock(b []byte) *Block {
	if b == nil || len(b) == 0 {
		return nil
	}
	cap := uint16(len(b))
	length := littleEndian.Uint16(b[0:2])

	return &Block{
		buf:    b,
		offset: 2,
		cap:    cap,
		length: length,
	}
}

func (block *Block) writeUint16(val uint16, offset uint16) {
	bbuf := make([]byte, 2)
	littleEndian.PutUint16(bbuf, val)
	copy(block.buf[offset:], bbuf)
}

func (block *Block) readUint16(offset uint16) uint16 {
	lbuf := block.buf[offset : offset+2]
	return littleEndian.Uint16(lbuf)
}

func (block *Block) write(b []byte) error {
	blen := uint16(len(b))
	if block.cap-block.offset < blen+4 {
		return errBlockLackOfSpace
	}
	block.writeUint16(blen, block.offset)
	block.offset += 2

	copy(block.buf[block.offset:], b)
	block.offset += blen
	block.length++
	block.writeUint16(block.length, 0)

	return nil
}

func (block *Block) read() ([]byte, error) {
	if block.offset >= block.cap || block.length == 0 {
		return nil, errBlockEmpty
	}

	blen := block.readUint16(block.offset)
	block.offset += 2

	if blen == 0 {
		return nil, errBlockEmpty
	}
	buf := make([]byte, blen)
	copy(buf, block.buf[block.offset:])
	block.offset += blen

	block.length--
	block.writeUint16(block.length, 0)

	return buf, nil
}

// 返回一个block.size 长度的数组，
func (block *Block) bytes() []byte {
	return block.buf
}

func (block *Block) reset() {
	block.offset = 2
	block.length = 0
	block.writeUint16(block.length, 0)
}

func (block *Block) hasSpace(space uint16) bool {
	return block.cap-block.offset >= space
}

// FileLog 用于记录数据
type FileLog struct {
	sync.Mutex
	writeblock int
	readblock  int
	file       *os.File
	writelog   chan []byte
	sub        func(log []*bytes.Buffer) error
	quit       chan struct{}
}

// Config Config
type Config struct {
	File    string
	SubFunc func(log []*bytes.Buffer) error
}

// NewFileLog 根据文件路径创建一个 FileLog
func NewFileLog(config *Config) (*FileLog, error) {
	f, err := os.OpenFile(config.File, os.O_APPEND|os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		return nil, err
	}

	fl := &FileLog{
		file:       f,
		writelog:   make(chan []byte),
		sub:        config.SubFunc,
		readblock:  int(readUint32(f, 0)),
		writeblock: int(readUint32(f, 4)),
		quit:       make(chan struct{}),
	}
	go fl.readloop()
	go fl.writeloop()

	return fl, nil
}

// Pub 写一条信息到文件
func (flog *FileLog) Write(log []byte) {
	flog.writelog <- log
}

func (flog *FileLog) writeloop() {
	t := time.NewTicker(time.Second)
	defer flog.file.Close()
	block := newBlock(make([]byte, blockSize))
	for {
		select {
		case wlog := <-flog.writelog:
			if !block.hasSpace(uint16(len(wlog) + 2)) {
				log.Println("append block to file, logs ", block.length)
				flog.appendBlock(block.bytes())
				block.reset()
			}
			err := block.write(wlog)
			if err != nil {
				log.Println(err)
			}
		case <-t.C:
			if block.length > 0 {
				log.Println("append block to file, logs ", block.length)
				flog.appendBlock(block.bytes())
				block.reset()
			}
			// log.Println(readUint32(flog.file, 0), readUint32(flog.file, 4))
		case <-flog.quit:
			return
		}
	}
}

func (flog *FileLog) appendBlock(b []byte) error {
	flog.Lock()
	offset := flog.writeblock*blockSize + 8

	_, err := flog.file.WriteAt(b, int64(offset))
	if err != nil {
		return err
	}
	flog.writeblock++
	err = writeUint32(flog.file, uint32(flog.writeblock), 4)
	if err != nil {
		flog.Unlock()
		return err
	}
	flog.Unlock()
	// flog.file.Sync()
	return nil
}

func (flog *FileLog) nextBlock() bool {
	flog.Lock()
	if flog.readblock == flog.writeblock {
		flog.Unlock()
		return false
	}
	flog.Unlock()
	return true
}

func (flog *FileLog) getBlock() ([]byte, error) {
	flog.Lock()
	if flog.readblock == flog.writeblock {
		flog.Unlock()
		return nil, errNoMoreBlock
	}
	offset := flog.readblock*blockSize + 8
	buf := make([]byte, blockSize)
	_, err := flog.file.ReadAt(buf, int64(offset))
	flog.Unlock()
	if err != nil {
		return nil, err
	}
	if buf[0] == 0 {
		return nil, errNoMoreBlock
	}
	return buf, nil
}

func (flog *FileLog) readloop() {
	for {
		if !flog.nextBlock() {
			time.Sleep(time.Millisecond * 100)
			continue
		}
		blockbuf, err := flog.getBlock()
		if err != nil {
			log.Println(err)
			time.Sleep(time.Millisecond * 100)
			continue
		}
		block := newBlock(blockbuf)

		blockLength := block.length
		list := make([]*bytes.Buffer, blockLength)

		for i := uint16(0); i < blockLength; i++ {
			buf, _ := block.read()
			list[i] = bytes.NewBuffer(buf)
		}
		if err := flog.sub(list); err != nil {
			log.Println(err)
			time.Sleep(time.Second)
			continue
		}

		flog.Lock()
		flog.readblock++
		if flog.readblock == flog.writeblock {
			flog.readblock = 0
			flog.writeblock = 0
			writeUint32(flog.file, 0, 4)

			flog.file.Truncate(8)
		}
		writeUint32(flog.file, uint32(flog.readblock), 0)
		flog.Unlock()
	}
}

// Close Close
func (flog *FileLog) Close() {
	flog.quit <- struct{}{}
}

func readUint32(file *os.File, offset int64) uint32 {
	buf := make([]byte, 4)
	n, err := file.ReadAt(buf, offset)
	if err != nil {
		return 0
	}
	if n == 4 {
		return littleEndian.Uint32(buf)
	}
	return 0
}

func writeUint32(file *os.File, val uint32, offset int64) error {
	buf := make([]byte, 4)
	littleEndian.PutUint32(buf, val)
	_, err := file.WriteAt(buf, offset)
	if err != nil {
		return err
	}
	return nil
}