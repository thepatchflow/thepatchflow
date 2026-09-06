package healstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// HealEvent is the ground-truth record of a self-healing event.
// Every healed request writes one of these. Over time, this file
// becomes the cross-customer "breaking-change index": which vendors
// broke, what schema migration fixed it, and whether the fix was
// verified by replaying the exact failing request.
type HealEvent struct {
	ID           string            `json:"id"`
	Vendor       string            `json:"vendor"`
	Endpoint     string            `json:"endpoint"`
	OldSchema    map[string]string `json:"old_schema"`
	NewSchema    map[string]string `json:"new_schema"`
	Patch        map[string]string `json:"patch"`
	Verified     bool              `json:"verified"`
	ReplayStatus int               `json:"replay_status"`
	PRURL        string            `json:"pr_url"`
	Timestamp    string            `json:"timestamp"`
}

// Store persists heal events as JSONL. It is safe for concurrent use.
type Store struct {
	mu    sync.Mutex
	path  string
	items []HealEvent
}

// DefaultPath is where heal events are persisted unless HEAL_DATA_PATH is set.
const DefaultPath = "data/heals.jsonl"

func New(path string) *Store {
	s := &Store{path: path}
	s.load()
	return s
}

func (s *Store) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev HealEvent
		if json.Unmarshal([]byte(line), &ev) == nil {
			s.items = append(s.items, ev)
		}
	}
}

// Record appends an event and persists it to disk.
func (s *Store) Record(ev HealEvent) HealEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ev.ID == "" {
		ev.ID = newID()
	}
	if ev.Timestamp == "" {
		ev.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	s.items = append(s.items, ev)
	s.persist()
	return ev
}

// UpdatePRURL back-fills the pull request URL on a previously recorded event.
func (s *Store) UpdatePRURL(id, prURL string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == id {
			s.items[i].PRURL = prURL
			break
		}
	}
	s.persist()
}

func (s *Store) persist() {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return
	}
	var buf strings.Builder
	for _, ev := range s.items {
		line, err := json.Marshal(ev)
		if err != nil {
			continue
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	os.WriteFile(s.path, []byte(buf.String()), 0o644)
}

// List returns recorded events, most recent first.
func (s *Store) List(limit int) []HealEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]HealEvent, len(s.items))
	copy(out, s.items)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamp > out[j].Timestamp
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// ByVendor returns events grouped by vendor for the breaking-change index.
type VendorStats struct {
	Vendor      string `json:"vendor"`
	TotalHeals  int    `json:"total_heals"`
	Verified    int    `json:"verified"`
	DistinctEndpoints []string `json:"distinct_endpoints"`
}

func (s *Store) Leaders() []VendorStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	byVendor := map[string]*VendorStats{}
	for _, ev := range s.items {
		vs, ok := byVendor[ev.Vendor]
		if !ok {
			vs = &VendorStats{Vendor: ev.Vendor}
			byVendor[ev.Vendor] = vs
		}
		vs.TotalHeals++
		if ev.Verified {
			vs.Verified++
		}
		if !contains(vs.DistinctEndpoints, ev.Endpoint) {
			vs.DistinctEndpoints = append(vs.DistinctEndpoints, ev.Endpoint)
		}
	}
	out := make([]VendorStats, 0, len(byVendor))
	for _, vs := range byVendor {
		out = append(out, *vs)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].TotalHeals > out[j].TotalHeals
	})
	return out
}

// Vendor infers the upstream API vendor from an endpoint path.
func Vendor(endpoint string) string {
	ep := strings.ToLower(endpoint)
	for _, v := range []string{"stripe", "salesforce", "paypal", "twilio", "github", "openai", "anthropic", "slack", "aws"} {
		if strings.Contains(ep, v) {
			return v
		}
	}
	ep = strings.Trim(ep, "/")
	segs := strings.Split(ep, "/")
	for len(segs) > 0 {
		seg := segs[0]
		if len(seg) > 1 && seg[0] == 'v' && isDigit(seg[1]) {
			segs = segs[1:]
			continue
		}
		if seg != "" {
			return seg
		}
		segs = segs[1:]
	}
	return "unknown"
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func newID() string {
	return time.Now().UTC().Format("20060102-150405.000000000")
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}