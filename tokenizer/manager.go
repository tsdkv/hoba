package tokenizer

import "sync"

// Manager maps tokenizer names (schema.Field.Tokenizer) to analyzers.
// It is an instance, not a global registry, and safe for concurrent use.
type Manager struct {
	mu     sync.RWMutex
	byName map[string]*Analyzer
}

// NewManager returns a Manager with the built-in analyzers registered:
// "default" (Simple → RemoveLong(40) → LowerCase) and "keyword" (Keyword
// alone). The names match what schema.TextField and schema.Keyword write.
func NewManager() *Manager {
	m := &Manager{byName: make(map[string]*Analyzer)}
	m.Register("default", NewAnalyzer(Simple{}, RemoveLong(40), LowerCase))
	m.Register("keyword", NewAnalyzer(Keyword{}))
	return m
}

// Register adds or replaces the analyzer for name.
func (m *Manager) Register(name string, a *Analyzer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byName[name] = a
}

// Get returns the analyzer registered under name.
func (m *Manager) Get(name string) (*Analyzer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.byName[name]
	return a, ok
}
