package analytics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	queueFileName = "analytics.queue"
	prefsFileName = "analytics.prefs.json"
	maxQueueSize  = 2000
)

// Event 单条埋点（与上报协议一致）
type Event struct {
	Schema     int                    `json:"schema"`
	Event      string                 `json:"event"`
	Ts         string                 `json:"ts"`
	InstallID  string                 `json:"installId"`
	AppVersion string                 `json:"appVersion"`
	OS         string                 `json:"os"`
	IsPro      bool                   `json:"isPro"`
	Props      map[string]interface{} `json:"props"`
}

type prefs struct {
	Enabled *bool `json:"enabled"`
}

type queueStore struct {
	mu   sync.Mutex
	path string
}

func newQueueStore(userDataDir string) *queueStore {
	return &queueStore{path: filepath.Join(userDataDir, queueFileName)}
}

func (q *queueStore) Append(ev Event) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	list, err := q.readUnlocked()
	if err != nil {
		return err
	}
	list = append(list, ev)
	if len(list) > maxQueueSize {
		list = list[len(list)-maxQueueSize:]
	}
	return q.writeUnlocked(list)
}

func (q *queueStore) Peek(n int) ([]Event, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	list, err := q.readUnlocked()
	if err != nil {
		return nil, err
	}
	if n <= 0 || n > len(list) {
		n = len(list)
	}
	out := make([]Event, n)
	copy(out, list[:n])
	return out, nil
}

func (q *queueStore) Drop(n int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	list, err := q.readUnlocked()
	if err != nil {
		return err
	}
	if n <= 0 {
		return nil
	}
	if n >= len(list) {
		return q.writeUnlocked(nil)
	}
	return q.writeUnlocked(list[n:])
}

func (q *queueStore) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	list, err := q.readUnlocked()
	if err != nil {
		return 0
	}
	return len(list)
}

func (q *queueStore) readUnlocked() ([]Event, error) {
	b, err := os.ReadFile(q.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return nil, nil
	}
	var list []Event
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (q *queueStore) writeUnlocked(list []Event) error {
	if list == nil {
		list = []Event{}
	}
	b, err := json.Marshal(list)
	if err != nil {
		return err
	}
	dir := filepath.Dir(q.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := q.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, q.path)
}

func loadEnabled(userDataDir string, defaultEnabled bool) bool {
	path := filepath.Join(userDataDir, prefsFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		return defaultEnabled
	}
	var p prefs
	if err := json.Unmarshal(b, &p); err != nil || p.Enabled == nil {
		return defaultEnabled
	}
	return *p.Enabled
}

func saveEnabled(userDataDir string, enabled bool) error {
	path := filepath.Join(userDataDir, prefsFileName)
	p := prefs{Enabled: &enabled}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(userDataDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func nowISO() string {
	return time.Now().Format(time.RFC3339)
}
