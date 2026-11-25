package component

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const eventMethod = "/demo.component.v1.Events/Stream"

type eventsService interface{ stream(grpc.ServerStream) error }
type mockEvents struct{ token string }

func (s mockEvents) stream(stream grpc.ServerStream) error {
	values := metadata.ValueFromIncomingContext(stream.Context(), "instance-token")
	if len(values) != 1 || values[0] != s.token || s.token == "" {
		return status.Error(codes.Unauthenticated, "invalid instance token")
	}
	var ready wrapperspb.StringValue
	if err := stream.RecvMsg(&ready); err != nil {
		return err
	}
	if ready.Value != "ready" {
		return status.Error(codes.InvalidArgument, "missing stream greeting")
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-ticker.C:
			if err := stream.SendMsg(wrapperspb.String("mock heartbeat")); err != nil {
				return err
			}
			var ack wrapperspb.StringValue
			if err := stream.RecvMsg(&ack); err != nil {
				return err
			}
			if ack.Value == "" {
				return status.Error(codes.InvalidArgument, "missing durable event ID")
			}
		}
	}
}
func registerEvents(server *grpc.Server, token string) {
	d := grpc.ServiceDesc{ServiceName: "demo.component.v1.Events", HandlerType: (*eventsService)(nil), Streams: []grpc.StreamDesc{{
		StreamName: "Stream", ServerStreams: true, ClientStreams: true,
		Handler: func(s any, stream grpc.ServerStream) error { return s.(eventsService).stream(stream) },
	}}}
	server.RegisterService(&d, mockEvents{token})
}
func Collect(ctx context.Context, conn *grpc.ClientConn, token string, commit func(string) (string, error)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "instance-token", token)
	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true, ClientStreams: true}, eventMethod, grpc.MaxCallRecvMsgSize(8192))
	if err != nil {
		return err
	}
	defer stream.CloseSend()
	// Send initial metadata so the service starts before the first ACK.
	if err = stream.SendMsg(wrapperspb.String("ready")); err != nil {
		return err
	}
	for {
		var payload wrapperspb.StringValue
		if err = stream.RecvMsg(&payload); err != nil {
			return err
		}
		id, err := commit(payload.Value)
		if err != nil {
			return err
		}
		if err = stream.SendMsg(wrapperspb.String(id)); err != nil {
			return err
		}
	}
}
