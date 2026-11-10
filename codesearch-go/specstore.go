package main

// Compiled tree-sitter spec cache: the single caching pattern for every
// language+query table in the tool (chunking defs, callers, caller defs,
// related exports, related vars, related Rust).
//
// Before this file, each table hand-rolled its own mutex + map +
// nil-caching logic (6 copies, 2 spec types). They are now 6 instances
// of one generic store. Instances must stay separate — never merge them
// into one global cache: the same extension maps to a different query
// in each table (e.g. ".js" is a def query for chunking, a call query
// for --callers, and an export query for --related), so a shared key
// space would serve the wrong query.
//
// Semantics preserved from the old copies: thread-safe; a failed build
// (unsupported extension, nil language, bad query) is cached as nil so
// it is not rebuilt on every file; get returns (nil, false) for a
// cached failure.
import "sync"

type specStore[T any] struct {
	mu sync.Mutex
	m  map[string]*T
}

func newSpecStore[T any]() *specStore[T] {
	return &specStore[T]{m: map[string]*T{}}
}

func (s *specStore[T]) get(key string, build func() (*T, bool)) (*T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.m[key]; ok {
		return v, v != nil
	}
	v, ok := build()
	if !ok || v == nil {
		s.m[key] = nil
		return nil, false
	}
	s.m[key] = v
	return v, true
}
