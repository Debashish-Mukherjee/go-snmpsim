package agent

import (
	"sync"
	"testing"

	"github.com/debashish-mukherjee/go-snmpsim/internal/store"
	"github.com/debashish-mukherjee/go-snmpsim/internal/v3"
)

func TestHandlePacketUpdatesPollStatsConcurrently(t *testing.T) {
	va := NewVirtualAgent(1, 20000, "device-1", store.NewOIDDatabase(), v3.Config{}, 1)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				_ = va.HandlePacketFrom([]byte{0x00}, nil, 20000)
				_ = va.GetStatistics()
			}
		}()
	}
	wg.Wait()

	stats := va.GetStatistics()
	if count, ok := stats["poll_count"].(int64); !ok || count == 0 {
		t.Fatalf("poll_count not updated: %v", stats["poll_count"])
	}
	if lastPoll, ok := stats["last_poll"].(string); !ok || lastPoll == "" {
		t.Fatalf("last_poll not populated: %v", stats["last_poll"])
	}
}
