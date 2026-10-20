package main

import (
	"sync"
	"time"
)

// cacheTTL is how long a fetched subject, season chain or episode list stays in memory. The host stores the scraped metadata; the
// plugin only avoids asking twice while one library is being scanned.
const (
	cacheTTL     = 30 * time.Minute
	maxCached    = 256
	maxChainSeas = 30
)

type cached[T any] struct {
	v   T
	exp time.Time
}

// cache holds subjects by ID, season groups (the subject IDs of each season) by root subject ID, and episode lists by subject ID and type.
type cache struct {
	mu       sync.Mutex
	subjects map[int]cached[*bgmSubject]
	chains   map[int]cached[[][]int]
	episodes map[[2]int]cached[[]bgmEpisode]
	now      func() time.Time // replaceable in tests
}

func (c *cache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func (c *cache) subject(id int) (*bgmSubject, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.subjects[id]
	if !ok || c.clock().After(e.exp) {
		return nil, false
	}
	return e.v, true
}

func (c *cache) putSubject(s *bgmSubject) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.subjects == nil || len(c.subjects) >= maxCached {
		c.subjects = map[int]cached[*bgmSubject]{}
	}
	c.subjects[s.ID] = cached[*bgmSubject]{s, c.clock().Add(cacheTTL)}
}

func (c *cache) chain(root int) ([][]int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.chains[root]
	if !ok || c.clock().After(e.exp) {
		return nil, false
	}
	return e.v, true
}

func (c *cache) putChain(root int, groups [][]int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.chains == nil || len(c.chains) >= maxCached {
		c.chains = map[int]cached[[][]int]{}
	}
	cp := make([][]int, len(groups))
	for i, g := range groups {
		cp[i] = append([]int(nil), g...)
	}
	c.chains[root] = cached[[][]int]{cp, c.clock().Add(cacheTTL)}
}

func (c *cache) episodeList(id, typ int) ([]bgmEpisode, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.episodes[[2]int{id, typ}]
	if !ok || c.clock().After(e.exp) {
		return nil, false
	}
	return e.v, true
}

func (c *cache) putEpisodes(id, typ int, list []bgmEpisode) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.episodes == nil || len(c.episodes) >= maxCached {
		c.episodes = map[[2]int]cached[[]bgmEpisode]{}
	}
	c.episodes[[2]int{id, typ}] = cached[[]bgmEpisode]{list, c.clock().Add(cacheTTL)}
}
