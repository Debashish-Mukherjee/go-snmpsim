package bench

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/debashish-mukherjee/go-snmpsim/internal/store"
	"github.com/gosnmp/gosnmp"
)

var benchOIDsPerAgent = 12

func BenchmarkStoreConcurrentGet(b *testing.B) {
	agentScales := []int{1000, 5000, 10000}
	for _, agents := range agentScales {
		db, oids := buildScaleDB(agents)
		b.Run(fmt.Sprintf("agents_%d", agents), func(b *testing.B) {
			b.SetParallelism(8)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				rng := rand.New(rand.NewSource(time.Now().UnixNano()))
				for pb.Next() {
					oid := oids[rng.Intn(len(oids))]
					_ = db.Get(oid)
				}
			})
		})
	}
}

func TestStoreLatencyProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("skip latency profile in short mode")
	}
	if os.Getenv("SNMPSIM_RUN_BENCHMARKS") != "1" {
		t.Skip("set SNMPSIM_RUN_BENCHMARKS=1 to run latency profile")
	}
	samples := 5000
	if raw := os.Getenv("SNMPSIM_BENCH_SAMPLES"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			samples = n
		}
	}
	workers := 16
	if raw := os.Getenv("SNMPSIM_BENCH_WORKERS"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			workers = n
		}
	}

	agentScales := []int{1000, 5000, 10000}
	for _, agents := range agentScales {
		db, oids := buildScaleDB(agents)
		lat := runConcurrentSamples(db, oids, samples, workers)
		p50, p95 := percentile(lat, 50), percentile(lat, 95)
		t.Logf("agents=%d samples=%d p50=%s p95=%s", agents, len(lat), p50, p95)
	}
}

func buildScaleDB(agentCount int) (*store.OIDDatabase, []string) {
	db := store.NewOIDDatabase()
	entries := make(map[string]*store.OIDValue, agentCount*benchOIDsPerAgent)
	oids := make([]string, 0, agentCount*benchOIDsPerAgent)

	for device := 1; device <= agentCount; device++ {
		prefix := "1.3.6.1.4.1.55555.1."
		for i := 1; i <= benchOIDsPerAgent; i++ {
			oid := prefix + strconv.Itoa(i) + "." + strconv.Itoa(device)
			entries[oid] = &store.OIDValue{Type: gosnmp.Integer, Value: uint32(device + i)}
			oids = append(oids, oid)
		}
	}
	db.BatchInsert(entries)
	db.SortOIDs()
	return db, oids
}

func runConcurrentSamples(db *store.OIDDatabase, oids []string, samples int, workers int) []time.Duration {
	latencies := make([]time.Duration, samples)
	jobs := make(chan int, samples)
	for i := 0; i < samples; i++ {
		jobs <- i
	}
	close(jobs)

	done := make(chan struct{}, workers)
	for worker := 0; worker < workers; worker++ {
		go func(seed int64) {
			rng := rand.New(rand.NewSource(seed))
			for idx := range jobs {
				start := time.Now()
				_ = db.Get(oids[rng.Intn(len(oids))])
				latencies[idx] = time.Since(start)
			}
			done <- struct{}{}
		}(time.Now().UnixNano() + int64(worker*97))
	}
	for i := 0; i < workers; i++ {
		<-done
	}
	return latencies
}

func percentile(samples []time.Duration, p int) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := (len(sorted) - 1) * p / 100
	return sorted[idx]
}
