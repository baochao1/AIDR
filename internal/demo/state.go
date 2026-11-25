package demo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	_ "modernc.org/sqlite"
)

type Desired struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	State    string `json:"desired_state"`
	Artifact string `json:"artifact_id"`
}
type Policy struct {
	Revision   uint64    `json:"revision"`
	Components []Desired `json:"components"`
}

func (p Policy) Validate() error {
	seen := map[string]bool{}
	if p.Revision == 0 || len(p.Components) > 64 {
		return errors.New("invalid revision or component count")
	}
	for _, d := range p.Components {
		if !valid(d.ID) || seen[d.ID] {
			return errors.New("invalid or duplicate component ID")
		}
		seen[d.ID] = true
		switch d.State {
		case "absent":
		case "running", "stopped":
			if !valid(d.Version) || !valid(d.Artifact) {
				return errors.New("version and artifact required")
			}
		default:
			return errors.New("invalid desired state")
		}
	}
	return nil
}

type Status struct {
	Version        string      `json:"version,omitempty"`
	State          string      `json:"state"`
	Error          string      `json:"error,omitempty"`
	PID            int         `json:"pid,omitempty"`
	IPC            string      `json:"ipc_transport,omitempty"`
	DesiredVersion string      `json:"desired_version,omitempty"`
	Data           *DataStatus `json:"data,omitempty"`
}
type DataStatus struct {
	State       string     `json:"state"`
	LastEventAt *time.Time `json:"last_event_at,omitempty"`
	Error       string     `json:"error,omitempty"`
}
type Report struct {
	CheckinAt  time.Time         `json:"checkin_at"`
	Received   uint64            `json:"received_revision"`
	Applied    uint64            `json:"applied_revision"`
	Components map[string]Status `json:"components"`
}
type Store struct{ *sql.DB }

func OpenStore(path string) (Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return Store{}, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL; CREATE TABLE IF NOT EXISTS state (key TEXT PRIMARY KEY,value BLOB NOT NULL)")
	if err != nil {
		db.Close()
		return Store{}, err
	}
	s := Store{db}
	if err := s.initEvents(); err != nil {
		db.Close()
		return Store{}, err
	}
	return s, nil
}
func (s Store) Get(key string, target any) error {
	var data []byte
	err := s.QueryRow("SELECT value FROM state WHERE key=?", key).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
func (s Store) Put(key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.Exec("INSERT INTO state VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, data)
	return err
}
