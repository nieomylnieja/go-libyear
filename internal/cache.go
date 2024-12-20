package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Masterminds/semver"
)

const defaultCacheFileName = "modules"

type modulesCache interface {
	Load(path string, version *semver.Version) (*Module, bool)
	Save(m *Module) error
}

type cachePersistenceLayer interface {
	Save(module persistedModule) error
	Load() ([]persistedModule, error)
}

func NewCache(filePath string) (*Cache, error) {
	persistence, err := newFilePersistence(filePath)
	if err != nil {
		return nil, err
	}
	cache := &Cache{
		Modules:     make(map[string]*Module),
		rwm:         sync.RWMutex{},
		persistence: persistence,
	}
	return cache, cache.loadFromPersistence()
}

type Cache struct {
	Modules     map[string]*Module
	rwm         sync.RWMutex
	persistence cachePersistenceLayer
}

func (c *Cache) Load(path string, version *semver.Version) (module *Module, loaded bool) {
	c.rwm.RLock()
	defer c.rwm.RUnlock()
	module, loaded = c.Modules[c.moduleHash(path, version)]
	return
}

func (c *Cache) Save(m *Module) error {
	if c.Has(m) {
		return nil
	}
	c.rwm.Lock()
	defer c.rwm.Unlock()
	// Second check if we were racing with another goroutine.
	// We can't use read locks now (deadlock).
	_, has := c.Modules[c.moduleHash(m.Path, m.Version)]
	if has {
		return nil
	}
	c.Modules[c.moduleHash(m.Path, m.Version)] = m
	if c.persistence == nil {
		return nil
	}
	return c.persistence.Save(persistedModule{
		Path:    m.Path,
		Version: m.Version,
		Time:    m.Time,
	})
}

func (c *Cache) Has(m *Module) bool {
	c.rwm.RLock()
	defer c.rwm.RUnlock()
	_, has := c.Modules[c.moduleHash(m.Path, m.Version)]
	return has
}

type persistedModule struct {
	Path    string          `json:"path"`
	Version *semver.Version `json:"version"`
	Time    time.Time       `json:"time"`
}

func (c *Cache) loadFromPersistence() error {
	if c.persistence == nil {
		return nil
	}
	c.rwm.Lock()
	defer c.rwm.Unlock()
	modules, err := c.persistence.Load()
	if err != nil {
		return err
	}
	for _, m := range modules {
		hash := c.moduleHash(m.Path, m.Version)
		if _, ok := c.Modules[hash]; ok {
			fmt.Fprintf(os.Stderr, "WARN: duplicate module entry detected: %v\n", m)
			continue
		}
		c.Modules[hash] = &Module{
			Path:    m.Path,
			Version: m.Version,
			Time:    m.Time,
		}
	}
	return nil
}

func (c *Cache) moduleHash(path string, version *semver.Version) string {
	return path + "=" + version.String()
}

func newFilePersistence(filePath string) (*filePersistence, error) {
	if filePath == "" {
		var err error
		if filePath, err = GetDefaultCacheBasePath(); err != nil {
			return nil, err
		}
		filePath = filepath.Join(filePath, defaultCacheFileName)
	}
	// The function does an os.Stat under the hood anyway, so there's no gain in pre-checking this step.
	if err := os.MkdirAll(filepath.Dir(filePath), 0o750); err != nil {
		return nil, err
	}
	// #nosec G304
	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	return &filePersistence{file: f}, nil
}

type filePersistence struct {
	file *os.File
}

func (f filePersistence) Save(module persistedModule) error {
	return json.NewEncoder(f.file).Encode(module)
}

func (f filePersistence) Load() ([]persistedModule, error) {
	dec := json.NewDecoder(f.file)
	modules := make([]persistedModule, 0)
	for {
		var m persistedModule
		if err := dec.Decode(&m); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		modules = append(modules, m)
	}
	return modules, nil
}

func GetDefaultCacheBasePath() (string, error) {
	filePath, envSet := os.LookupEnv("XDG_CACHE_HOME")
	if envSet {
		return filepath.Join(filePath, ProgramName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", ProgramName), nil
}
