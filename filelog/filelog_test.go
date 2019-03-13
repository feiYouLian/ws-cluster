package filelog

import (
	"bytes"
	"fmt"
	"log"
	"testing"
	"time"
)

func sub(logs []*bytes.Buffer) error {
	fmt.Println("sub ", len(logs))
	// for index := 0; index < len(logs); index++ {
	// 	fmt.Println(logs[index].Bytes())
	// }
	// time.Sleep(time.Millisecond * 300)
	return nil
}

func TestNewFileLog(t *testing.T) {
	quit := make(chan bool)
	msgCount := 50000
	recv := 0
	filelog, err := NewFileLog(&Config{
		File: "./msg.log",
		SubFunc: func(logs []*bytes.Buffer) error {
			recv += len(logs)
			log.Println("recv:", recv)
			if recv == msgCount {
				quit <- true
			}
			time.Sleep(time.Millisecond * 100)
			return nil
		},
	})
	if err != nil {
		return
	}

	go func() {
		for index := 0; index < msgCount; index++ {
			buf := make([]byte, 2)
			littleEndian.PutUint16(buf, uint16(index))
			log := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
			copy(log, buf)
			filelog.Write(log)
		}
	}()

	<-quit

	// time.Sleep(time.Second * 5)

}
