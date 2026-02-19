package v3

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type EngineState struct {
	EngineID string `json:"engine_id"`
	Boots    uint32 `json:"boots"`
	Updated  int64  `json:"updated"`
}

type EngineStateStore struct {
	path  string
	mu    sync.Mutex
	state map[string]EngineState
}

func NewEngineStateStore(path string) (*EngineStateStore, error) {
	if path == "" {
		path = filepath.Join(os.TempDir(), "go-snmpsim-engine-state.json")
	}
	store := &EngineStateStore{path: path, state: map[string]EngineState{}}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func GenerateEngineID(seed string) string {
	if seed == "" {
		seed = fmt.Sprintf("snmpsim-%d", time.Now().UnixNano())
	}
	h := sha1.Sum([]byte(seed))
	// enterprise prefix + deterministic suffix
	return string(append([]byte{0x80, 0x00, 0x1F, 0x88}, h[:12]...))
}

func ParseEngineID(input string) (string, error) {
	if input == "" {
		return "", nil
	}
	clean := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(input)), "0x")
	if decoded, err := hex.DecodeString(clean); err == nil {
		return string(decoded), nil
	}
	return input, nil
}

func (s *EngineStateStore) EnsureBoots(engineID string) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mapKey := hex.EncodeToString([]byte(engineID))

	st, ok := s.state[mapKey]
	if !ok {
		st = EngineState{EngineID: mapKey, Boots: 1, Updated: time.Now().Unix()}
	} else {
		st.EngineID = mapKey
		st.Boots++
		st.Updated = time.Now().Unix()
	}
	s.state[mapKey] = st
	return st.Boots, s.save()
}

func (s *EngineStateStore) load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(b, &s.state)
}

func (s *EngineStateStore) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o600)
}
