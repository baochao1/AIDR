package component

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	pb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func Serve(listener net.Listener, token string, healthy bool) error {
	options := []grpc.ServerOption{grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		values := metadata.ValueFromIncomingContext(ctx, "instance-token")
		if token == "" || len(values) != 1 || values[0] != token {
			return nil, status.Error(codes.Unauthenticated, "invalid instance token")
		}
		return next(ctx, req)
	})}
	if cert := os.Getenv("DEMO_IPC_CERT"); cert != "" {
		pair, err := tls.X509KeyPair([]byte(cert), []byte(os.Getenv("DEMO_IPC_KEY")))
		if err != nil {
			return err
		}
		options = append(options, grpc.Creds(credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair},
		})))
	}
	server := grpc.NewServer(options...)
	svc := health.NewServer()
	state := pb.HealthCheckResponse_SERVING
	if !healthy {
		state = pb.HealthCheckResponse_NOT_SERVING
	}
	svc.SetServingStatus("", state)
	pb.RegisterHealthServer(server, svc)
	registerEvents(server, token)
	return server.Serve(listener)
}

func Probe(ctx context.Context, client pb.HealthClient, token string) error {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	response, err := client.Check(metadata.AppendToOutgoingContext(ctx, "instance-token", token), &pb.HealthCheckRequest{}, grpc.WaitForReady(true))
	if err != nil {
		return err
	}
	if response.Status != pb.HealthCheckResponse_SERVING {
		return errors.New("component not serving")
	}
	return nil
}

func Dial(endpoint string) (*grpc.ClientConn, error) {
	return grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

func DialPinned(endpoint string, certificate []byte, native bool) (*grpc.ClientConn, error) {
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certificate) {
		return nil, errors.New("invalid component certificate")
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "127.0.0.1"}
	options := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(cfg))}
	if native {
		options = append(options, grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return dialNative(ctx, endpoint)
		}))
		return grpc.NewClient("passthrough:///localhost", options...)
	}
	return grpc.NewClient(endpoint, options...)
}

func Mock() error {
	if os.Getenv("DEMO_PARENT_PIPE") == "1" {
		go func() { _, _ = io.Copy(io.Discard, os.Stdin); os.Exit(0) }()
	}
	endpoint := os.Getenv("DEMO_ENDPOINT")
	var listener net.Listener
	var err error
	switch os.Getenv("DEMO_IPC_TRANSPORT") {
	case NativeTransport:
		listener, err = listenNative(endpoint)
	case "", "tcp":
		listener, err = net.Listen("tcp", endpoint)
	default:
		return errors.New("unsupported component transport")
	}
	if err != nil {
		return err
	}
	defer listener.Close()
	return Serve(listener, os.Getenv("DEMO_TOKEN"), os.Getenv("DEMO_VERSION") != "3.0.0")
}
