package wire

import (
	"log"
	"testing"
)

func TestGetOutboundIP(t *testing.T) {
	log.Println(GetOutboundIP())
}
