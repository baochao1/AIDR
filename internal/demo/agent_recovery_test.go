package demo

import (
	"context"
	"net"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestCollectorReconnectsAndPreservesSequence(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var sessions atomic.Int32
	received := make(chan string, 2)
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
		session := sessions.Add(1)
		var ready, ack wrapperspb.StringValue
		if err := stream.RecvMsg(&ready); err != nil {
			return err
		}
		if ready.Value != "ready" {
			return status.Error(codes.InvalidArgument, "expected ready")
		}
		if err := stream.SendMsg(wrapperspb.String("heartbeat")); err != nil {
			return err
		}
		if err := stream.RecvMsg(&ack); err != nil {
			return err
		}
		received <- ack.Value
		if session == 1 {
			return status.Error(codes.Unavailable, "interrupted test stream")
		}
		<-stream.Context().Done()
		return nil
	}))
	go server.Serve(listener)
	defer server.Stop()
	conn, err := grpc.NewClient("passthrough:///test",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	p := &child{conn: conn, token: "test-token"}
	a := coordinator{store: store}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	done := make(chan struct{})
	go func() {
		defer close(done)
		a.collect(ctx, "mock-a", p)
	}()
	defer func() { cancel(); <-done }()
	var ids []string
	for range 2 {
		select {
		case id := <-received:
			ids = append(ids, id)
		case <-ctx.Done():
			t.Fatal("did not resume event stream")
		}
	}
	_, events, err := store.batch()
	if err != nil || len(events) != 2 {
		t.Fatalf("stored events: %v, %v", events, err)
	}
	for i, event := range events {
		if event.Sequence != uint64(i+1) || event.ID != ids[i] {
			t.Fatalf("unexpected record for ACK %q: %+v", ids[i], event)
		}
	}
	data := p.data.Load()
	if data == nil || data.State != "receiving" || data.LastEventAt == nil {
		t.Fatalf("unexpected data status: %+v", data)
	}
}

func TestRestartBudgetExpires(t *testing.T) {
	now := time.Now()
	a := coordinator{
		failures: map[string][]time.Time{}, lastFault: map[string]string{}, next: map[string]time.Time{},
	}
	for i := range restartLimit {
		a.recordFault("mock-a", "exited", now.Add(time.Duration(i)*time.Second))
	}
	if count := len(a.restartFailures("mock-a", now.Add(5*time.Second))); count != restartLimit {
		t.Fatalf("restart limit not reached: %d", count)
	}
	if count := len(a.restartFailures("mock-a", now.Add(restartWindow))); count != restartLimit-1 {
		t.Fatalf("oldest failure did not expire: %d", count)
	}
}
