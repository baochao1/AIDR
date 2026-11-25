package demo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/klauspost/compress/zstd"
)

const maxQueuedEvents = 4096
const maxBatchEvents = 128
const maxBatchBytes = 1 << 20

var errQueueFull = errors.New("event queue full")

type Event struct {
	ID        string    `json:"event_id"`
	Component string    `json:"component_id"`
	Instance  string    `json:"instance_id"`
	Schema    int       `json:"schema_version"`
	At        time.Time `json:"timestamp"`
	Payload   string    `json:"payload"`
	Sequence  uint64    `json:"sequence"`
}

func (s Store) initEvents() error {
	_, err := s.Exec(`CREATE TABLE IF NOT EXISTS queue (seq INTEGER PRIMARY KEY AUTOINCREMENT, body BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS event_receipts (agent TEXT NOT NULL,event_id TEXT NOT NULL,body BLOB NOT NULL,PRIMARY KEY(agent,event_id));`)
	return err
}
func (s Store) enqueue(e Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	result, err := s.Exec("INSERT INTO queue(body) SELECT ? WHERE (SELECT COUNT(*) FROM queue) < ?", body, maxQueuedEvents)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err == nil && rows != 1 {
		return errQueueFull
	}
	return err
}
func (s Store) batch() (int64, []Event, error) {
	rows, err := s.Query("SELECT seq, body FROM queue ORDER BY seq LIMIT ?", maxBatchEvents)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	var high int64
	var batch []Event
	for rows.Next() {
		var data []byte
		if err = rows.Scan(&high, &data); err != nil {
			return 0, nil, err
		}
		var event Event
		if err = json.Unmarshal(data, &event); err != nil {
			return 0, nil, err
		}
		batch = append(batch, event)
	}
	return high, batch, rows.Err()
}
func (s Store) ingest(agent string, batch []Event) error {
	if len(batch) == 0 || len(batch) > maxBatchEvents {
		return errors.New("invalid event batch size")
	}
	tx, err := s.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, e := range batch {
		if !valid(e.Component) || !valid(e.Instance) || !valid(e.ID) || e.Schema != 1 || e.At.IsZero() || e.Sequence < 1 || len(e.Payload) > 1024 {
			return errors.New("invalid event")
		}
		body, err := json.Marshal(e)
		if err != nil {
			return err
		}
		_, err = tx.Exec("INSERT INTO event_receipts VALUES(?,?,?) ON CONFLICT(agent,event_id) DO NOTHING", agent, e.ID, body)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
func receiveEvents(w http.ResponseWriter, r *http.Request, s Store, agent string) error {
	if r.Header.Get("Content-Encoding") != "zstd" {
		return errors.New("zstd content encoding required")
	}
	compressed := http.MaxBytesReader(w, r.Body, maxBatchBytes)
	reader, err := zstd.NewReader(compressed, zstd.WithDecoderMaxMemory(8<<20), zstd.WithDecoderConcurrency(1))
	if err != nil {
		return err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, maxBatchBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxBatchBytes {
		return errors.New("event batch exceeds decoded size limit")
	}
	var batch []Event
	if err = Decode(bytes.NewReader(data), &batch); err != nil {
		return err
	}
	return s.ingest(agent, batch)
}
func (s Store) flushEvents(ctx context.Context, c Client) error {
	high, batch, err := s.batch()
	if err != nil || len(batch) == 0 {
		return err
	}
	body, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
	if err != nil {
		return err
	}
	compressed := encoder.EncodeAll(body, nil)
	encoder.Close()
	req, err := http.NewRequestWithContext(ctx, "POST", c.URL+"/v1/agents/events", bytes.NewReader(compressed))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Encoding", "zstd")
	req.Header.Set("Content-Type", "application/json")
	response, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("ingest HTTP %d", response.StatusCode)
	}
	_, err = s.Exec("DELETE FROM queue WHERE seq <= ?", high)
	return err
}
