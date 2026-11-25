package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"time"

	plugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	pb "google.golang.org/grpc/health/grpc_health_v1"
	"supervisor-demo/internal/component"
)

type healthPlugin struct{ plugin.NetRPCUnsupportedPlugin }

func (healthPlugin) GRPCServer(_ *plugin.GRPCBroker, _ *grpc.Server) error {
	return nil
}
func (healthPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return pb.NewHealthClient(c), nil
}

func run() error {
	handshake := plugin.HandshakeConfig{ProtocolVersion: 1, MagicCookieKey: "DEMO_POC", MagicCookieValue: "health"}
	plugins := plugin.PluginSet{"health": healthPlugin{}}
	if len(os.Args) > 1 && os.Args[1] == "child" {
		plugin.Serve(&plugin.ServeConfig{HandshakeConfig: handshake, Plugins: plugins, GRPCServer: plugin.DefaultGRPCServer})
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	start := time.Now()
	p := plugin.NewClient(&plugin.ClientConfig{HandshakeConfig: handshake, Plugins: plugins, Cmd: exec.Command(self, "child"), AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC}, AutoMTLS: true})
	defer p.Kill()
	rpc, err := p.Client()
	if err != nil {
		return err
	}
	service, err := rpc.Dispense("health")
	if err != nil {
		return err
	}
	for i := 0; i < 10; i++ {
		if err := component.Probe(context.Background(), service.(pb.HealthClient), ""); err != nil {
			return err
		}
	}
	fmt.Printf("go-plugin AutoMTLS: 10 checks OK in %v\n", time.Since(start))
	if len(os.Args) < 2 {
		return fmt.Errorf("provide mock-go binary, or python executable and script")
	}
	start = time.Now()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	address := listener.Addr().String()
	listener.Close()
	token := "poc-instance-token"
	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Env = append(os.Environ(), "DEMO_ENDPOINT="+address, "DEMO_TOKEN="+token)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Wait()
	defer cmd.Process.Kill()
	conn, err := component.Dial(address)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := pb.NewHealthClient(conn)
	deadline, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		err := component.Probe(deadline, client, token)
		if err == nil {
			break
		}
		if deadline.Err() != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	for i := 0; i < 10; i++ {
		if err := component.Probe(context.Background(), client, token); err != nil {
			return err
		}
	}
	if err := component.Probe(context.Background(), client, "wrong"); err == nil {
		return fmt.Errorf("wrong token accepted")
	}
	fmt.Printf("native gRPC: 10 checks OK and invalid token rejected in %v\n", time.Since(start))
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
