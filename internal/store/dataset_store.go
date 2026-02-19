package store

import (
	"fmt"
	"strings"
)

type DatasetStore struct {
	defaultPath string
	datasets    map[string]*OIDDatabase
	indexes     map[string]*OIDIndexManager
}

func NewDatasetStore(defaultPath string, extraPaths []string) (*DatasetStore, error) {
	paths := make([]string, 0, len(extraPaths)+1)
	paths = append(paths, defaultPath)
	paths = append(paths, extraPaths...)

	unique := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		unique = append(unique, p)
	}

	store := &DatasetStore{
		defaultPath: strings.TrimSpace(defaultPath),
		datasets:    make(map[string]*OIDDatabase, len(unique)),
		indexes:     make(map[string]*OIDIndexManager, len(unique)),
	}

	for _, path := range unique {
		db, err := LoadOIDDatabase(path)
		if err != nil {
			return nil, fmt.Errorf("load dataset %q: %w", path, err)
		}
		idx := NewOIDIndexManager()
		if err := idx.BuildIndex(db); err != nil {
			return nil, fmt.Errorf("build index for dataset %q: %w", path, err)
		}

		store.datasets[path] = db
		store.indexes[path] = idx
	}

	return store, nil
}

func (ds *DatasetStore) Resolve(path string) (*OIDDatabase, *OIDIndexManager) {
	if ds == nil {
		return nil, nil
	}

	path = strings.TrimSpace(path)
	if path != "" {
		if db, ok := ds.datasets[path]; ok {
			return db, ds.indexes[path]
		}
	}

	if db, ok := ds.datasets[ds.defaultPath]; ok {
		return db, ds.indexes[ds.defaultPath]
	}

	if db, ok := ds.datasets[""]; ok {
		return db, ds.indexes[""]
	}

	return nil, nil
}
