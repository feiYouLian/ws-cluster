package database

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/core"
	"github.com/go-xorm/xorm"
)

// MysqMessageStore mysql message store
type MysqMessageStore struct {
	engine *xorm.Engine
}

// NewMysqlMessageStore new a MysqMessageStore
func NewMysqlMessageStore(engine *xorm.Engine) *MysqMessageStore {
	return &MysqMessageStore{
		engine: engine,
	}
}

// Save save message to mysql
func (s *MysqMessageStore) Save(chatMsg *ChatMsg) error {

	return nil
}

// InitDb init database
func InitDb(ip string, port int, user, pwd, dbname string) *xorm.Engine {
	url := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local", user, pwd, ip, port, dbname)
	engine, err := xorm.NewEngine("mysql", url)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// engine.ShowSQL(true)

	tbMapper := core.NewPrefixMapper(core.SnakeMapper{}, "t_")
	engine.SetTableMapper(tbMapper)

	engine.SetColumnMapper(core.SnakeMapper{})

	// err = engine.Sync2(new(UserTaskRecord))
	// if err != nil {
	// 	fmt.Println(err)
	// }
	return engine
}
