package demo

import (
	"path/filepath"
	"testing"
	"time"
)

func TestEventIngestDeduplicates(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	event := Event{"demo-instance-1", "mock-a", "demo-instance", 1, time.Now(), "heartbeat", 1}
	for range 2 {
		if err := store.ingest("demo-01", []Event{event}); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := store.QueryRow("SELECT count(*) FROM event_receipts").Scan(&count); err != nil || count != 1 {
		t.Fatalf("expected one stored event, got %d (%v)", count, err)
	}
}
