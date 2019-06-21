package database

import (
	"fmt"
	"testing"
	"time"
)

func TestMemGroupCache_GetGroupMembers(t *testing.T) {
	group := NewMemGroupCache()
	for index := 0; index < 1000; index++ {
		group.Join("test", fmt.Sprintf("user %v", index))
	}

	for index := 0; index < 500; index++ {
		group.Leave("test", fmt.Sprintf("user %v", index))
	}

	time.Sleep(time.Second)

	users, _ := group.GetGroupMembers("test")
	if len(users) != 500 {
		t.Error("GetGroupMembers ", len(users))
	}

}
