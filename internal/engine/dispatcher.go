package engine

import (
	"sync"

	"github.com/debashish/go-snmpsim/internal/agent"
)

// PacketDispatcher routes incoming UDP packets to virtual agents
type PacketDispatcher struct {
	bufferPool *sync.Pool
}

// NewPacketDispatcher creates a new packet dispatcher
func NewPacketDispatcher(bufferPool *sync.Pool) *PacketDispatcher {
	return &PacketDispatcher{
		bufferPool: bufferPool,
	}
}

// Dispatch handles packet routing (currently handled at simulator level)
func (pd *PacketDispatcher) Dispatch(port int, packet []byte, a *agent.VirtualAgent) []byte {
	// Agent handles the packet and returns response
	return a.HandlePacket(packet)
}

// RecycleBuffer returns a buffer to the pool
func (pd *PacketDispatcher) RecycleBuffer(buf []byte) {
	if bufCap := cap(buf); bufCap == 4096 { // Only recycle standard-sized buffers
		pd.bufferPool.Put(buf)
	}
}
