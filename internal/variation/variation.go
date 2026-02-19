package variation

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
)

var (
	ErrDropOID = errors.New("drop oid")
	ErrTimeout = errors.New("timeout")
)

type PDU = gosnmp.SnmpPDU

type Variation interface {
	Apply(time.Time, PDU) (PDU, error)
}

func toInt64(v interface{}) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case uint:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		if x > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(x), true
	default:
		return 0, false
	}
}

func castByType(ber gosnmp.Asn1BER, n int64) interface{} {
	switch ber {
	case gosnmp.Counter32, gosnmp.Gauge32, gosnmp.TimeTicks, gosnmp.Uinteger32:
		if n < 0 {
			n = 0
		}
		return uint32(n)
	case gosnmp.Counter64:
		if n < 0 {
			n = 0
		}
		return uint64(n)
	default:
		return n
	}
}

type CounterMonotonic struct {
	Delta int64

	mu      sync.Mutex
	current map[string]int64
}

func NewCounterMonotonic(delta int64) *CounterMonotonic {
	if delta == 0 {
		delta = 1
	}
	return &CounterMonotonic{Delta: delta, current: map[string]int64{}}
}

func (v *CounterMonotonic) Apply(_ time.Time, pdu PDU) (PDU, error) {
	base, ok := toInt64(pdu.Value)
	if !ok {
		return pdu, nil
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	cur, exists := v.current[pdu.Name]
	if !exists {
		cur = base
	}
	cur += v.Delta
	v.current[pdu.Name] = cur

	pdu.Value = castByType(pdu.Type, cur)
	return pdu, nil
}

type RandomJitter struct {
	Max int64

	mu  sync.Mutex
	rng *rand.Rand
}

func NewRandomJitter(max int64, seed int64) *RandomJitter {
	if max < 0 {
		max = -max
	}
	if seed == 0 {
		seed = 1
	}
	return &RandomJitter{Max: max, rng: rand.New(rand.NewSource(seed))}
}

func (v *RandomJitter) Apply(_ time.Time, pdu PDU) (PDU, error) {
	if v.Max == 0 {
		return pdu, nil
	}
	base, ok := toInt64(pdu.Value)
	if !ok {
		return pdu, nil
	}
	v.mu.Lock()
	delta := v.rng.Int63n(v.Max*2+1) - v.Max
	v.mu.Unlock()
	pdu.Value = castByType(pdu.Type, base+delta)
	return pdu, nil
}

type Step struct {
	Period time.Duration
	Delta  int64

	mu      sync.Mutex
	base    map[string]int64
	startAt map[string]time.Time
}

func NewStep(period time.Duration, delta int64) *Step {
	if period <= 0 {
		period = time.Second
	}
	return &Step{Period: period, Delta: delta, base: map[string]int64{}, startAt: map[string]time.Time{}}
}

func (v *Step) Apply(now time.Time, pdu PDU) (PDU, error) {
	base, ok := toInt64(pdu.Value)
	if !ok {
		return pdu, nil
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	b, exists := v.base[pdu.Name]
	if !exists {
		v.base[pdu.Name] = base
		v.startAt[pdu.Name] = now
		b = base
	}
	steps := int64(now.Sub(v.startAt[pdu.Name]) / v.Period)
	pdu.Value = castByType(pdu.Type, b+steps*v.Delta)
	return pdu, nil
}

type PeriodicReset struct {
	Period time.Duration

	mu       sync.Mutex
	base     map[string]int64
	windowAt map[string]time.Time
	current  map[string]int64
}

func NewPeriodicReset(period time.Duration) *PeriodicReset {
	if period <= 0 {
		period = 5 * time.Minute
	}
	return &PeriodicReset{
		Period:   period,
		base:     map[string]int64{},
		windowAt: map[string]time.Time{},
		current:  map[string]int64{},
	}
}

func (v *PeriodicReset) Apply(now time.Time, pdu PDU) (PDU, error) {
	base, ok := toInt64(pdu.Value)
	if !ok {
		return pdu, nil
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	_, exists := v.base[pdu.Name]
	if !exists {
		v.base[pdu.Name] = base
		v.current[pdu.Name] = base
		v.windowAt[pdu.Name] = now
	}

	if now.Sub(v.windowAt[pdu.Name]) >= v.Period {
		v.current[pdu.Name] = v.base[pdu.Name]
		v.windowAt[pdu.Name] = now
	} else {
		v.current[pdu.Name]++
	}

	pdu.Value = castByType(pdu.Type, v.current[pdu.Name])
	return pdu, nil
}

type DropOID struct{}

func (v *DropOID) Apply(_ time.Time, pdu PDU) (PDU, error) {
	return pdu, ErrDropOID
}

type Timeout struct {
	Delay time.Duration
}

func (v *Timeout) Apply(_ time.Time, pdu PDU) (PDU, error) {
	if v.Delay > 0 {
		time.Sleep(v.Delay)
	}
	return pdu, ErrTimeout
}

type Chain []Variation

func (c Chain) Apply(now time.Time, pdu PDU) (PDU, error) {
	var err error
	for _, v := range c {
		pdu, err = v.Apply(now, pdu)
		if err != nil {
			return pdu, err
		}
	}
	return pdu, nil
}

func ParseDuration(value string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return 0, fmt.Errorf("empty duration")
	}
	return time.ParseDuration(value)
}
