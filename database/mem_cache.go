package database

import "time"

// MemGroupCache 群缓存
type MemGroupCache struct {
	groups map[string]*Group
	event  chan oper
}

type oper struct {
	op       int8 // 1 join 2 leave
	groups   []string
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
func (c *MemGroupCache) Join(group string, clientID string) {
	c.event <- oper{1, []string{group}, clientID}
}

// Leave 退出群
func (c *MemGroupCache) Leave(group string, clientID string) {
	c.event <- oper{2, []string{group}, clientID}
}

// JoinMany 加入一个群，如果群不存在就创建一个
func (c *MemGroupCache) JoinMany(clientID string, groups []string) {
	c.event <- oper{1, groups, clientID}
}

// LeaveMany 退出群
func (c *MemGroupCache) LeaveMany(clientID string, groups []string) {
	c.event <- oper{2, groups, clientID}
}

// GetGroupMembers 取群中成员
func (c *MemGroupCache) GetGroupMembers(group string) []string {
	g, ok := c.groups[group]
	if !ok || len(g.Clients) == 0 {
		return nil
	}
	mems := make([]string, len(g.Clients))
	index := 0
	for key := range g.Clients {
		mems[index] = key
		index++
	}
	return mems
}

// GetGroups GetGroups
func (c *MemGroupCache) GetGroups() []string {
	glen := len(c.groups)
	groups := make([]string, glen)
	index := 0
	for key := range c.groups {
		groups[index] = key
		index++
	}
	return groups
}

func handleEvent(c *MemGroupCache) {
	ticker := time.NewTicker(time.Hour)

	for {
		select {
		case ev := <-c.event:
			groups := ev.groups
			if len(groups) == 0 {
				continue
			}
			clientID := ev.clientID
			if ev.op == 1 { //join
				for _, group := range groups {
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
				}
			} else if ev.op == 2 {
				for _, group := range groups {
					if _, ok := c.groups[group]; !ok {
						continue
					}
					g := c.groups[group]

					if _, ok := g.Clients[clientID]; ok {
						delete(g.Clients, clientID)
					}
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
