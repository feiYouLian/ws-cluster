package database

import (
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/core"
	"github.com/go-xorm/xorm"
)

var (
	// ErrInsertFail data insert affected zeo
	ErrInsertFail = fmt.Errorf("data insert fail")
)

// MysqMessageStore mysql message store
type MysqMessageStore struct {
	engine *xorm.Engine
}

// NewMysqlMessageStore new a MysqMessageStore
func NewMysqlMessageStore(engine *xorm.Engine) *MysqMessageStore {
	if engine == nil {
		return &MysqMessageStore{}
	}
	err := engine.Sync2(new(ChatMsg))
	if err != nil {
		log.Println(err)
	}

	return &MysqMessageStore{
		engine: engine,
	}
}

// Save save message to mysql
func (s *MysqMessageStore) Save(chatMsgs ...*ChatMsg) error {
	if s.engine == nil {
		return nil
	}
	aff, err := s.engine.Insert(chatMsgs)
	if err != nil {
		return err
	}
	if aff == 0 {
		return ErrInsertFail
	}
	return nil
}

// InitDb init database
func InitDb(ip string, port int, user, pwd, dbname string) *xorm.Engine {
	url := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local", user, pwd, ip, port, dbname)
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
