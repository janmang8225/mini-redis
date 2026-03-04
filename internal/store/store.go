package store

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

// ValueType identifies what kind of data is stored at a key.
type ValueType uint8

const (
	TypeString ValueType = iota
	TypeList
	TypeHash
	TypeSet
)

// entry is the internal representation of a stored value.
type entry struct {
	valueType ValueType
	value     any       // string | []string | map[string]string | map[string]struct{}
	expiresAt time.Time // zero value means no expiry
}

func (e *entry) isExpired() bool {
	if e.expiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.expiresAt)
}

// Store is the thread-safe in-memory key-value store.
// RWMutex: multiple readers OR one writer at a time.
type Store struct {
	mu   sync.RWMutex
	data map[string]*entry
}

func New() *Store {
	return &Store{
		data: make(map[string]*entry),
	}
}

// --- low-level helpers used by command handlers ---

// get returns the entry for a key, or nil if it doesn't exist / is expired.
// Caller must hold at least a read lock.
func (s *Store) get(key string) *entry {
	e, ok := s.data[key]
	if !ok {
		return nil
	}
	if e.isExpired() {
		// lazy deletion — we'll clean it up on the next write or active expiry
		return nil
	}
	return e
}

// GetString returns the string value for a key and whether it existed.
func (s *Store) GetString(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e := s.get(key)
	if e == nil {
		return "", false
	}
	if e.valueType != TypeString {
		return "", false
	}
	return e.value.(string), true
}

// SetString sets a string value. ttl=0 means no expiry.
func (s *Store) SetString(key, value string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e := &entry{
		valueType: TypeString,
		value:     value,
	}
	if ttl > 0 {
		e.expiresAt = time.Now().Add(ttl)
	}
	s.data[key] = e
}

// Delete removes keys and returns how many existed.
func (s *Store) Delete(keys ...string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	var deleted int64
	for _, key := range keys {
		if e, ok := s.data[key]; ok && !e.isExpired() {
			delete(s.data, key)
			deleted++
		}
	}
	return deleted
}

// Exists returns how many of the given keys exist (and are not expired).
func (s *Store) Exists(keys ...string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, key := range keys {
		if e := s.get(key); e != nil {
			count++
		}
	}
	return count
}

// DBSize returns the number of keys (including potentially expired ones not yet cleaned).
func (s *Store) DBSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// FlushAll removes all keys.
func (s *Store) FlushAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]*entry)
}

// GetType returns the type string for a key, like Redis TYPE command.
func (s *Store) GetType(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e := s.get(key)
	if e == nil {
		return "none"
	}
	switch e.valueType {
	case TypeString:
		return "string"
	case TypeList:
		return "list"
	case TypeHash:
		return "hash"
	case TypeSet:
		return "set"
	default:
		return "none"
	}
}

// Expire sets a TTL on an existing key. Returns true if key existed.
func (s *Store) Expire(key string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok || e.isExpired() {
		return false
	}
	e.expiresAt = time.Now().Add(ttl)
	return true
}

// IncrBy atomically increments a string integer value by delta.
func (s *Store) IncrBy(key string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	var current int64

	if ok && !e.isExpired() {
		if e.valueType != TypeString {
			return 0, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		n, err := strconv.ParseInt(e.value.(string), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("value is not an integer or out of range")
		}
		current = n
	}

	newVal := current + delta
	s.data[key] = &entry{
		valueType: TypeString,
		value:     strconv.FormatInt(newVal, 10),
	}
	return newVal, nil
}

// TTL returns the remaining TTL for a key.
// Returns -2 if key doesn't exist, -1 if key exists but has no expiry.
func (s *Store) TTL(key string) time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e := s.get(key)
	if e == nil {
		return -2
	}
	if e.expiresAt.IsZero() {
		return -1
	}
	return time.Until(e.expiresAt)
}