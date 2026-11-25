package demo

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"runtime"
	"testing"
)

func TestArtifactValidation(t *testing.T) {
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	m := Manifest{"mock-a", "1.0.0", runtime.GOOS, runtime.GOARCH, "bin/component", "process", 1, ""}
	d := Desired{"mock-a", "1.0.0", "running", "mock-a-1"}
	pack := func(m Manifest) Artifact {
		t.Helper()
		a, err := Sign(d.Artifact, m, []byte("test executable"), key)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	root := t.TempDir()
	a := pack(m)
	path, err := a.Install(root, pub, d)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path.executable()); err != nil || string(data) != "test executable" {
		t.Fatalf("installed bytes: %q, %v", data, err)
	}
	a.ZIP[0] ^= 1
	if _, err = a.Install(root, pub, d); err == nil {
		t.Fatal("tampered ZIP accepted")
	}
	bad := m
	bad.Entrypoint = "../outside"
	if _, err = pack(bad).Install(root, pub, d); err == nil {
		t.Fatal("path traversal accepted")
	}
	bad = m
	bad.OS = "other"
	if _, err = pack(bad).Install(root, pub, d); err == nil {
		t.Fatal("wrong platform accepted")
	}
	if err := os.Chmod(path.executable(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path.executable(), []byte("altered executable"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := path.verify(); err == nil {
		t.Fatal("modified executable accepted for launch")
	}
	if _, err := pack(m).Install(root, pub, d); err == nil {
		t.Fatal("modified cached package accepted for reuse")
	}
}
