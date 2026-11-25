//go:build linux || darwin

package component

import (
	"context"
	"net"
	"os"
	"path/filepath"
)

const NativeTransport = "unix"

func NativeEndpoint(_ string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "sd-")
	if err != nil {
		return "", nil, err
	}
	socket := filepath.Join(dir, "ipc.sock")
	return socket, func() {
		_ = os.Remove(socket)
		_ = os.Remove(dir)
	}, nil
}

func listenNative(endpoint string) (net.Listener, error) {
	return net.Listen("unix", endpoint)
}

func dialNative(ctx context.Context, endpoint string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", endpoint)
}
