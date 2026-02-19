package variation

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Binder struct {
	bindings []prefixChain
}

type prefixChain struct {
	prefix string
	chain  Chain
}

type binderConfig struct {
	Bindings []bindingSpec `yaml:"bindings"`
}

type bindingSpec struct {
	Prefix     string          `yaml:"prefix"`
	Variations []variationSpec `yaml:"variations"`
}

type variationSpec struct {
	Type   string `yaml:"type"`
	Delta  int64  `yaml:"delta"`
	Max    int64  `yaml:"max"`
	Seed   int64  `yaml:"seed"`
	Period string `yaml:"period"`
	Delay  string `yaml:"delay"`
}

func NewBinder(specs []bindingSpec) (*Binder, error) {
	out := make([]prefixChain, 0, len(specs))
	for i, spec := range specs {
		prefix := normalizeOIDPrefix(spec.Prefix)
		if prefix == "" {
			return nil, fmt.Errorf("binding %d: prefix is required", i)
		}
		chain := make(Chain, 0, len(spec.Variations))
		for j, vs := range spec.Variations {
			v, err := buildVariation(vs)
			if err != nil {
				return nil, fmt.Errorf("binding %d variation %d: %w", i, j, err)
			}
			chain = append(chain, v)
		}
		out = append(out, prefixChain{prefix: prefix, chain: chain})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return len(out[i].prefix) > len(out[j].prefix)
	})

	return &Binder{bindings: out}, nil
}

func LoadBinder(path string) (*Binder, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read variation file: %w", err)
	}
	var cfg binderConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse variation yaml: %w", err)
	}
	return NewBinder(cfg.Bindings)
}

func (b *Binder) Apply(now time.Time, pdu PDU) (PDU, error) {
	if b == nil {
		return pdu, nil
	}
	oid := normalizeOIDPrefix(pdu.Name)
	for _, entry := range b.bindings {
		if matchesPrefix(oid, entry.prefix) {
			return entry.chain.Apply(now, pdu)
		}
	}
	return pdu, nil
}

func matchesPrefix(oid, prefix string) bool {
	if oid == prefix {
		return true
	}
	return strings.HasPrefix(oid, prefix+".")
}

func normalizeOIDPrefix(oid string) string {
	oid = strings.TrimSpace(oid)
	oid = strings.TrimPrefix(oid, ".")
	return oid
}

func buildVariation(spec variationSpec) (Variation, error) {
	switch strings.ToLower(strings.TrimSpace(spec.Type)) {
	case "countermonotonic":
		return NewCounterMonotonic(spec.Delta), nil
	case "randomjitter":
		return NewRandomJitter(spec.Max, spec.Seed), nil
	case "step":
		d, err := ParseDuration(spec.Period)
		if err != nil {
			return nil, fmt.Errorf("invalid period: %w", err)
		}
		return NewStep(d, spec.Delta), nil
	case "periodicreset":
		d, err := ParseDuration(spec.Period)
		if err != nil {
			return nil, fmt.Errorf("invalid period: %w", err)
		}
		return NewPeriodicReset(d), nil
	case "dropoid":
		return &DropOID{}, nil
	case "timeout":
		d, err := ParseDuration(spec.Delay)
		if err != nil {
			return nil, fmt.Errorf("invalid delay: %w", err)
		}
		return &Timeout{Delay: d}, nil
	default:
		return nil, fmt.Errorf("unsupported variation type %q", spec.Type)
	}
}
