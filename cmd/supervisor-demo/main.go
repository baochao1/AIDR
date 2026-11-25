package main

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"supervisor-demo/internal/demo"
)

func readJSON(path string, target any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return demo.Decode(f, target)
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("expected bootstrap, server, agent, sign, publish, policy or status")
	}
	command := os.Args[1]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	root := flags.String("root", ".demo", "state directory")
	identity := flags.String("identity", "", "identity directory (defaults to root/command)")
	addr := flags.String("listen", "127.0.0.1:8443", "server listen address")
	url := flags.String("url", "https://127.0.0.1:8443", "management URL")
	agent := flags.String("agent", "demo-01", "agent ID")
	file := flags.String("file", "", "policy, artifact or executable file")
	id := flags.String("id", "", "component ID")
	version := flags.String("version", "", "component version")
	artifactID := flags.String("artifact", "", "artifact ID")
	ipc := flags.String("ipc", "", "component IPC: native (named pipe/Unix socket), tcp, or empty for legacy TCP")
	output := flags.String("out", "", "signed artifact output")
	events := flags.Bool("events", false, "collect and upload synthetic demo events")
	if err := flags.Parse(os.Args[2:]); err != nil {
		return err
	}
	dir := *identity
	if dir == "" {
		role := command
		switch command {
		case "publish", "policy", "status":
			role = "admin"
		case "sign":
			role = "signer"
		}
		dir = filepath.Join(*root, role)
	}
	switch command {
	case "bootstrap":
		return bootstrap(*root)
	case "sign":
		key, err := os.ReadFile(filepath.Join(dir, "sign.key"))
		if err != nil {
			return err
		}
		f, err := os.Open(*file)
		if err != nil {
			return err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > demo.MaxPackage {
			return errors.New("invalid executable size or type")
		}
		data, err := os.ReadFile(*file)
		if err != nil {
			return err
		}
		entry := "bin/component"
		if runtime.GOOS == "windows" {
			entry += ".exe"
		}
		m := demo.Manifest{ID: *id, Version: *version, OS: runtime.GOOS, Arch: runtime.GOARCH, Entrypoint: entry, Runtime: "process", Protocol: 1, IPC: *ipc}
		artifact, err := demo.Sign(*artifactID, m, data, ed25519.PrivateKey(key))
		if err != nil {
			return err
		}
		data, err = json.Marshal(artifact)
		if err != nil {
			return err
		}
		return os.WriteFile(*output, data, 0600)
	case "publish", "policy", "status":
		c, err := demo.NewClient(*url, dir, "admin")
		if err != nil {
			return err
		}
		path := "/v1/admin/agents/" + *agent
		switch command {
		case "publish":
			var a demo.Artifact
			if err := readJSON(*file, &a); err != nil {
				return err
			}
			return c.Request("POST", "/v1/admin/artifacts", a, nil)
		case "policy":
			var p demo.Policy
			if err := readJSON(*file, &p); err != nil {
				return err
			}
			return c.Request("PUT", path+"/policy", p, nil)
		default:
			var report demo.Report
			if err := c.Request("GET", path, nil, &report); err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(report)
		}
	case "server", "agent":
		key, err := os.ReadFile(filepath.Join(dir, "sign.pub"))
		if err != nil {
			return err
		}
		if len(key) != ed25519.PublicKeySize {
			return errors.New("invalid signing public key")
		}
		token, err := os.ReadFile(filepath.Join(dir, "enrollment"))
		if err != nil {
			return err
		}
		stateDir, err := filepath.Abs(filepath.Join(*root, command))
		if err != nil {
			return err
		}
		if err = os.MkdirAll(stateDir, 0700); err != nil {
			return err
		}
		lock, err := demo.Lock(filepath.Join(stateDir, "owner.lock"))
		if err != nil {
			return fmt.Errorf("state directory locked or inaccessible: %w", err)
		}
		defer lock.Close()
		store, err := demo.OpenStore(filepath.Join(stateDir, "state.db"))
		if err != nil {
			return err
		}
		defer store.Close()
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		if command == "agent" {
			c, err := demo.NewClient(*url, dir, "agent")
			if err != nil {
				return err
			}
			return demo.RunAgent(ctx, store, c, key, stateDir, strings.TrimSpace(string(token)), *events)
		}
		registry := filepath.Join(stateDir, "registry")
		if err = os.MkdirAll(registry, 0700); err != nil {
			return err
		}
		tlsConfig, err := demo.TLS(dir, "server")
		if err != nil {
			return err
		}
		s := http.Server{Addr: *addr, TLSConfig: tlsConfig, Handler: demo.Server(store, registry, key, strings.TrimSpace(string(token))), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 25 * time.Second, WriteTimeout: 25 * time.Second}
		go func() { <-ctx.Done(); _ = s.Close() }()
		err = s.ListenAndServeTLS("", "")
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
