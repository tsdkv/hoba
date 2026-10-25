package tokenizer

import "sync"

type Manager struct {
	mu     sync.RWMutex
	byName map[string]*Analyzer
}

// The built-in names match what schema.TextField and schema.Keyword write.
func NewManager() *Manager {
	m := &Manager{byName: make(map[string]*Analyzer)}
	m.Register("default", NewAnalyzer(Simple{}, RemoveLong(40), LowerCase))
	m.Register("keyword", NewAnalyzer(Keyword{}))
	return m
}

func (m *Manager) Register(name string, a *Analyzer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byName[name] = a
}

func (m *Manager) Get(name string) (*Analyzer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.byName[name]
	return a, ok
}
