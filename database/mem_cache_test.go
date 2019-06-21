package database

import (
	"fmt"
	"testing"
	"time"
)

func TestMemGroupCache_GetGroupMembers1(t *testing.T) {
	group := NewMemGroupCache()
	for index := 0; index < 1000; index++ {
		group.Join("test", fmt.Sprintf("user %v", index))
	}

	for index := 0; index < 500; index++ {
		group.Leave("test", fmt.Sprintf("user %v", index))
	}

	time.Sleep(time.Second)

	users := group.GetGroupMembers("test")
	if len(users) != 500 {
		t.Error("GetGroupMembers ", len(users))
	}

}

func TestMemGroupCache_GetGroupMembers2(t *testing.T) {
	group := NewMemGroupCache()
	groupsjoin := make([]string, 1000)
	for index := 0; index < 1000; index++ {
		groupsjoin[index] = fmt.Sprintf("group_%v", index)
	}
	group.JoinMany("test", groupsjoin)

	groupsleave := make([]string, 500)
	for index := 0; index < 500; index++ {
		groupsleave[index] = fmt.Sprintf("group_%v", index)
	}
	group.LeaveMany("test", groupsleave)

	time.Sleep(time.Second)

}
