package database

import (
	"fmt"
	"log"

	// just init
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	"xorm.io/core"
)

var (
	// ErrInsertFail data insert affected zeo
	ErrInsertFail = fmt.Errorf("data insert fail")
)

// DbMessageStore mysql message store
type DbMessageStore struct {
	engine *xorm.Engine
}

// NewDbMessageStore new a DbMessageStore
func NewDbMessageStore(engine *xorm.Engine) *DbMessageStore {
	if engine == nil {
		return &DbMessageStore{}
	}
	err := engine.Sync2(new(ChatMsg), new(GroupMsg))
	if err != nil {
		log.Println(err)
	}
	return &DbMessageStore{
		engine: engine,
	}
}

// SaveChatMsg save message to mysql
func (s *DbMessageStore) SaveChatMsg(msgs []*ChatMsg) error {
	if s.engine == nil {
		return nil
	}
	_, err := s.engine.Insert(msgs)
	if err != nil {
		return err
	}
	return nil
}

// SaveGroupMsg SaveGroupMsg
func (s *DbMessageStore) SaveGroupMsg(msgs []*GroupMsg) error {
	if s.engine == nil {
		return nil
	}
	_, err := s.engine.Insert(msgs)
	if err != nil {
		return err
	}
	return nil
}

// InitMysqlDb init mysql database
func InitMysqlDb(source string) *xorm.Engine {
	url := fmt.Sprintf("%s?charset=utf8&parseTime=True&loc=Local", source)
	// url := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local", user, pwd, ip, port, dbname)
	engine, err := xorm.NewEngine("mysql", url)
	if err != nil {
		log.Println(err)
		return nil
	}

	// engine.ShowSQL(true)

	tbMapper := core.NewPrefixMapper(core.SnakeMapper{}, "t_")
	engine.SetTableMapper(tbMapper)

	engine.SetColumnMapper(core.SnakeMapper{})

	return engine
}
