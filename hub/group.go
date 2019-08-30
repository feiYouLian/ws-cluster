package hub

import "github.com/ws-cluster/wire"

const (
	useForJoin    = uint8(1)
	useForLeave   = uint8(3)
	useForMessage = uint8(5)
)

// GroupPacket use for Join Leave message
type GroupPacket struct {
	use     uint8
	content interface{}
}

// Group Group
type Group struct {
	Addr     wire.Addr
	Members  map[wire.Addr]*ClientPeer
	MemCount int
	packet   chan *GroupPacket
	exit     chan struct{}
}

// NewGroup NewGroup
func NewGroup(addr wire.Addr) *Group {
	group := &Group{
		Addr:    addr,
		Members: make(map[wire.Addr]*ClientPeer),
		packet:  make(chan *GroupPacket, 50),
		exit:    make(chan struct{}, 1),
	}
	go group.loop()

	return group
}

func (g *Group) loop() {
	for {
		select {
		case packet := <-g.packet:
			if packet.use == useForMessage {
				message := packet.content.(*wire.Message)
				for _, peer := range g.Members {
					peer.PushMessage(message, nil)
				}
			} else {
				peer := packet.content.(*ClientPeer)
				if packet.use == useForJoin {
					g.Members[peer.Addr] = peer
					g.MemCount++
				} else {
					delete(g.Members, peer.Addr)
					g.MemCount--
				}
			}
		case <-g.exit:
			return
		}
	}
}

// Exit stop group loop
func (g *Group) Exit() {
	g.exit <- struct{}{}
}
