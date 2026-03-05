package store

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/janmang8225/mini-redis/internal/persistence"
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

// ─── Lists ─────────────────────────────────────────────────────────────────────

// LPush prepends values to a list. Returns new length.
func (s *Store) LPush(key string, values ...string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if ok && !e.isExpired() && e.valueType != TypeList {
		return 0, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	var list []string
	if ok && !e.isExpired() {
		list = e.value.([]string)
	}

	// prepend each value (like Redis — last arg ends up at head)
	for _, v := range values {
		list = append([]string{v}, list...)
	}

	s.data[key] = &entry{valueType: TypeList, value: list}
	return int64(len(list)), nil
}

// RPush appends values to a list. Returns new length.
func (s *Store) RPush(key string, values ...string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if ok && !e.isExpired() && e.valueType != TypeList {
		return 0, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	var list []string
	if ok && !e.isExpired() {
		list = e.value.([]string)
	}

	list = append(list, values...)
	s.data[key] = &entry{valueType: TypeList, value: list}
	return int64(len(list)), nil
}

// LPop removes and returns the first element of a list.
func (s *Store) LPop(key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e := s.get(key)
	if e == nil {
		return "", false, nil
	}
	if e.valueType != TypeList {
		return "", false, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	list := e.value.([]string)
	if len(list) == 0 {
		return "", false, nil
	}

	val := list[0]
	list = list[1:]
	if len(list) == 0 {
		delete(s.data, key)
	} else {
		e.value = list
	}
	return val, true, nil
}

// RPop removes and returns the last element of a list.
func (s *Store) RPop(key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e := s.get(key)
	if e == nil {
		return "", false, nil
	}
	if e.valueType != TypeList {
		return "", false, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	list := e.value.([]string)
	if len(list) == 0 {
		return "", false, nil
	}

	val := list[len(list)-1]
	list = list[:len(list)-1]
	if len(list) == 0 {
		delete(s.data, key)
	} else {
		e.value = list
	}
	return val, true, nil
}

// LRange returns elements from start to stop (inclusive). Negative indices supported.
func (s *Store) LRange(key string, start, stop int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e := s.get(key)
	if e == nil {
		return []string{}, nil
	}
	if e.valueType != TypeList {
		return nil, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	list := e.value.([]string)
	n := len(list)

	// normalize negative indices
	if start < 0 {
		start = n + start
	}
	if stop < 0 {
		stop = n + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if start > stop {
		return []string{}, nil
	}

	return list[start : stop+1], nil
}

// LLen returns the length of a list.
func (s *Store) LLen(key string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e := s.get(key)
	if e == nil {
		return 0, nil
	}
	if e.valueType != TypeList {
		return 0, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	return int64(len(e.value.([]string))), nil
}

// ─── Hashes ────────────────────────────────────────────────────────────────────

// HSet sets field-value pairs on a hash. Returns number of new fields added.
func (s *Store) HSet(key string, pairs map[string]string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if ok && !e.isExpired() && e.valueType != TypeHash {
		return 0, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	var hash map[string]string
	if ok && !e.isExpired() {
		hash = e.value.(map[string]string)
	} else {
		hash = make(map[string]string)
	}

	var added int64
	for f, v := range pairs {
		if _, exists := hash[f]; !exists {
			added++
		}
		hash[f] = v
	}

	s.data[key] = &entry{valueType: TypeHash, value: hash}
	return added, nil
}

// HGet returns the value of a hash field.
func (s *Store) HGet(key, field string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e := s.get(key)
	if e == nil {
		return "", false, nil
	}
	if e.valueType != TypeHash {
		return "", false, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	val, ok := e.value.(map[string]string)[field]
	return val, ok, nil
}

// HDel removes fields from a hash. Returns number deleted.
func (s *Store) HDel(key string, fields ...string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e := s.get(key)
	if e == nil {
		return 0, nil
	}
	if e.valueType != TypeHash {
		return 0, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	hash := e.value.(map[string]string)
	var deleted int64
	for _, f := range fields {
		if _, ok := hash[f]; ok {
			delete(hash, f)
			deleted++
		}
	}
	return deleted, nil
}

// HGetAll returns all field-value pairs in a hash.
func (s *Store) HGetAll(key string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e := s.get(key)
	if e == nil {
		return map[string]string{}, nil
	}
	if e.valueType != TypeHash {
		return nil, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// return a copy — caller shouldn't mutate store internals
	hash := e.value.(map[string]string)
	result := make(map[string]string, len(hash))
	for k, v := range hash {
		result[k] = v
	}
	return result, nil
}

// HLen returns the number of fields in a hash.
func (s *Store) HLen(key string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e := s.get(key)
	if e == nil {
		return 0, nil
	}
	if e.valueType != TypeHash {
		return 0, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	return int64(len(e.value.(map[string]string))), nil
}

// ─── Sets ──────────────────────────────────────────────────────────────────────

// SAdd adds members to a set. Returns number of new members added.
func (s *Store) SAdd(key string, members ...string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if ok && !e.isExpired() && e.valueType != TypeSet {
		return 0, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	var set map[string]struct{}
	if ok && !e.isExpired() {
		set = e.value.(map[string]struct{})
	} else {
		set = make(map[string]struct{})
	}

	var added int64
	for _, m := range members {
		if _, exists := set[m]; !exists {
			set[m] = struct{}{}
			added++
		}
	}

	s.data[key] = &entry{valueType: TypeSet, value: set}
	return added, nil
}

// SMembers returns all members of a set.
func (s *Store) SMembers(key string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e := s.get(key)
	if e == nil {
		return []string{}, nil
	}
	if e.valueType != TypeSet {
		return nil, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	set := e.value.(map[string]struct{})
	members := make([]string, 0, len(set))
	for m := range set {
		members = append(members, m)
	}
	return members, nil
}

// SRem removes members from a set. Returns number removed.
func (s *Store) SRem(key string, members ...string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e := s.get(key)
	if e == nil {
		return 0, nil
	}
	if e.valueType != TypeSet {
		return 0, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	set := e.value.(map[string]struct{})
	var removed int64
	for _, m := range members {
		if _, ok := set[m]; ok {
			delete(set, m)
			removed++
		}
	}
	return removed, nil
}

// SIsMember returns whether a value is a member of a set.
func (s *Store) SIsMember(key, member string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e := s.get(key)
	if e == nil {
		return false, nil
	}
	if e.valueType != TypeSet {
		return false, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	_, ok := e.value.(map[string]struct{})[member]
	return ok, nil
}

// SCard returns the number of members in a set.
func (s *Store) SCard(key string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e := s.get(key)
	if e == nil {
		return 0, nil
	}
	if e.valueType != TypeSet {
		return 0, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	return int64(len(e.value.(map[string]struct{}))), nil
}

// ─── Expiry ────────────────────────────────────────────────────────────────────

// DeleteExpired scans all keys and removes expired ones.
// Called by the active expiry worker — not by command handlers.
func (s *Store) DeleteExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int
	for key, e := range s.data {
		if e.isExpired() {
			delete(s.data, key)
			count++
		}
	}
	return count
}

// Keys returns all non-expired keys matching a simple pattern.
// Supports * (match all) and prefix* patterns only for now.
func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.data))
	for k, e := range s.data {
		if !e.isExpired() {
			keys = append(keys, k)
		}
	}
	return keys
}

// ─── Persistence ───────────────────────────────────────────────────────────────

// Snapshot exports the entire store as a map of SnapshotEntry for serialization.
func (s *Store) Snapshot() map[string]persistence.SnapshotEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]persistence.SnapshotEntry, len(s.data))
	for k, e := range s.data {
		if e.isExpired() {
			continue
		}
		entry := persistence.SnapshotEntry{
			Type:      uint8(e.valueType),
			ExpiresAt: e.expiresAt,
		}
		switch e.valueType {
		case TypeString:
			entry.StrVal = e.value.(string)
		case TypeList:
			list := e.value.([]string)
			entry.ListVal = make([]string, len(list))
			copy(entry.ListVal, list)
		case TypeHash:
			hash := e.value.(map[string]string)
			entry.HashVal = make(map[string]string, len(hash))
			for hk, hv := range hash {
				entry.HashVal[hk] = hv
			}
		case TypeSet:
			set := e.value.(map[string]struct{})
			members := make([]string, 0, len(set))
			for m := range set {
				members = append(members, m)
			}
			entry.SetVal = members
		}
		out[k] = entry
	}
	return out
}

// LoadSnapshot imports snapshot entries into the store.
// Called once on startup — no locking needed before server accepts connections.
func (s *Store) LoadSnapshot(entries map[string]persistence.SnapshotEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, se := range entries {
		e := &entry{
			valueType: ValueType(se.Type),
			expiresAt: se.ExpiresAt,
		}
		switch ValueType(se.Type) {
		case TypeString:
			e.value = se.StrVal
		case TypeList:
			e.value = se.ListVal
		case TypeHash:
			e.value = se.HashVal
		case TypeSet:
			set := make(map[string]struct{}, len(se.SetVal))
			for _, m := range se.SetVal {
				set[m] = struct{}{}
			}
			e.value = set
		}
		s.data[k] = e
	}
}

// ReplayCommand re-executes a single AOF command directly on the store.
// This bypasses the TCP/RESP layer — pure store operations only.
// Write commands only — reads are never logged to AOF.
func (s *Store) ReplayCommand(args []string) error {
	if len(args) == 0 {
		return nil
	}

	cmd := strings.ToUpper(args[0])
	switch cmd {
	case "SET":
		if len(args) >= 3 {
			s.SetString(args[1], args[2], 0)
		}
	case "DEL":
		if len(args) >= 2 {
			s.Delete(args[1:]...)
		}
	case "EXPIRE":
		if len(args) == 3 {
			var secs int64
			fmt.Sscanf(args[2], "%d", &secs)
			s.Expire(args[1], time.Duration(secs)*time.Second)
		}
	case "LPUSH":
		if len(args) >= 3 {
			s.LPush(args[1], args[2:]...)
		}
	case "RPUSH":
		if len(args) >= 3 {
			s.RPush(args[1], args[2:]...)
		}
	case "LPOP":
		if len(args) == 2 {
			s.LPop(args[1])
		}
	case "RPOP":
		if len(args) == 2 {
			s.RPop(args[1])
		}
	case "HSET":
		if len(args) >= 4 {
			pairs := make(map[string]string)
			for i := 2; i < len(args)-1; i += 2 {
				pairs[args[i]] = args[i+1]
			}
			s.HSet(args[1], pairs)
		}
	case "HDEL":
		if len(args) >= 3 {
			s.HDel(args[1], args[2:]...)
		}
	case "SADD":
		if len(args) >= 3 {
			s.SAdd(args[1], args[2:]...)
		}
	case "SREM":
		if len(args) >= 3 {
			s.SRem(args[1], args[2:]...)
		}
	case "FLUSHALL":
		s.FlushAll()
	case "INCR", "INCRBY", "DECR", "DECRBY":
		var delta int64 = 1
		if cmd == "DECR" {
			delta = -1
		}
		if (cmd == "INCRBY" || cmd == "DECRBY") && len(args) == 3 {
			fmt.Sscanf(args[2], "%d", &delta)
			if cmd == "DECRBY" {
				delta = -delta
			}
		}
		if len(args) >= 2 {
			s.IncrBy(args[1], delta)
		}
	}
	return nil
}
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