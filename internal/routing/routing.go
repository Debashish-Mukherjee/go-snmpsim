package routing

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Matchers struct {
	Community string `yaml:"community"`
	Context   string `yaml:"context"`
	EngineID  string `yaml:"engineID"`
	SrcIP     string `yaml:"srcIP"`
	DstPort   int    `yaml:"dstPort"`
}

type Action struct {
	DatasetPath string `yaml:"datasetPath"`
}

type Rule struct {
	Match  Matchers `yaml:"match"`
	Action Action   `yaml:"action"`
}

type Config struct {
	Routes []Rule `yaml:"routes"`
}

type RequestKey struct {
	Community string
	Context   string
	EngineID  string
	SrcIP     string
	DstPort   int
}

type Router struct {
	routes []Rule
}

func NewRouter(rules []Rule) (*Router, error) {
	validated := make([]Rule, 0, len(rules))
	for i, rule := range rules {
		if strings.TrimSpace(rule.Action.DatasetPath) == "" {
			return nil, fmt.Errorf("route %d: action.datasetPath is required", i)
		}
		validated = append(validated, rule)
	}

	sort.SliceStable(validated, func(i, j int) bool {
		pi := rulePriority(validated[i].Match)
		pj := rulePriority(validated[j].Match)
		return pi > pj
	})

	return &Router{routes: validated}, nil
}

func LoadFromFile(path string) (*Router, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read route file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse route yaml: %w", err)
	}

	return NewRouter(cfg.Routes)
}

func (r *Router) Select(key RequestKey) string {
	if r == nil {
		return ""
	}
	for _, rule := range r.routes {
		if ruleMatches(rule.Match, key) {
			return rule.Action.DatasetPath
		}
	}
	return ""
}

func (r *Router) DatasetPaths() []string {
	if r == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(r.routes))
	out := make([]string, 0, len(r.routes))
	for _, rule := range r.routes {
		path := strings.TrimSpace(rule.Action.DatasetPath)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func ruleMatches(m Matchers, key RequestKey) bool {
	if m.Community != "" && m.Community != key.Community {
		return false
	}
	if m.Context != "" && m.Context != key.Context {
		return false
	}
	if m.EngineID != "" && m.EngineID != key.EngineID {
		return false
	}
	if m.SrcIP != "" && m.SrcIP != key.SrcIP {
		return false
	}
	if m.DstPort != 0 && m.DstPort != key.DstPort {
		return false
	}
	return true
}

func rulePriority(m Matchers) int {
	if m.EngineID != "" && m.Context != "" {
		return 5
	}
	if m.Context != "" {
		return 4
	}
	if m.Community != "" {
		return 3
	}
	if m.SrcIP != "" || m.DstPort != 0 {
		return 2
	}
	return 1
}
