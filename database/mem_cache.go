package database

import "time"

// MemGroupCache 群缓存
type MemGroupCache struct {
	groups map[string]*Group
	event  chan oper
}

type oper struct {
	op       int8 // 1 join 2 leave
	group    string
	clientID string
}

// NewMemGroupCache NewMemGroupCache
func NewMemGroupCache() *MemGroupCache {
	c := &MemGroupCache{
		groups: make(map[string]*Group),
		event:  make(chan oper, 0), // no buffer channal
	}
	go handleEvent(c)

	return c
}

// Join 加入一个群，如果群不存在就创建一个
func (c *MemGroupCache) Join(group string, clientID string) error {
	c.event <- oper{1, group, clientID}
	return nil
}

// Leave 退出群
func (c *MemGroupCache) Leave(group string, clientID string) error {
	c.event <- oper{2, group, clientID}
	return nil
}

// GetGroupMembers 取群中成员
func (c *MemGroupCache) GetGroupMembers(group string) ([]string, error) {
	g, ok := c.groups[group]
	if !ok || len(g.Clients) == 0 {
		return nil, nil
	}
	mems := make([]string, len(g.Clients))
	index := 0
	for key := range g.Clients {
		mems[index] = key
		index++
	}
	return mems, nil
}

func handleEvent(c *MemGroupCache) {
	ticker := time.NewTicker(time.Hour)

	for {
		select {
		case ev := <-c.event:
			group := ev.group
			clientID := ev.clientID
			if ev.op == 1 { //join
				g, ok := c.groups[group]
				if !ok {
					c.groups[group] = &Group{
						Name:    group,
						Clients: make(map[string]bool, 100),
					}
					g = c.groups[group]
				}
				if _, ok := g.Clients[clientID]; !ok {
					g.Clients[clientID] = true
				}
			} else if ev.op == 2 {
				if _, ok := c.groups[group]; !ok {
					continue
				}
				g := c.groups[group]

				if _, ok := g.Clients[clientID]; ok {
					delete(g.Clients, clientID)
				}
			}
		case <-ticker.C:
			for name, group := range c.groups {
				if len(group.Clients) == 0 {
					delete(c.groups, name)
				}
			}
		}
	}
}
