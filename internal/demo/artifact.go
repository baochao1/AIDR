package demo

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const MaxPackage = 32 << 20

var identifier = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,63}$`)

func valid(s string) bool {
	if !identifier.MatchString(s) || strings.HasSuffix(s, ".") {
		return false
	}
	base := strings.Split(s, ".")[0]
	switch base {
	case "con", "prn", "aux", "nul", "com0", "com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9", "lpt0", "lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9":
		return false
	}
	return true
}

type Manifest struct {
	ID         string `json:"id"`
	Version    string `json:"version"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	Entrypoint string `json:"entrypoint"`
	Runtime    string `json:"runtime"`
	Protocol   int    `json:"protocol_version"`
	IPC        string `json:"ipc_transport,omitempty"`
}
type Descriptor struct {
	ArtifactID string   `json:"artifact_id"`
	Manifest   Manifest `json:"manifest"`
	SHA256     string   `json:"sha256"`
	Size       int      `json:"size"`
	KeyID      string   `json:"key_id"`
}
type Artifact struct {
	Descriptor Descriptor `json:"descriptor"`
	Signature  []byte     `json:"signature"`
	ZIP        []byte     `json:"zip"`
}

func Sign(id string, m Manifest, content []byte, key ed25519.PrivateKey) (Artifact, error) {
	if len(key) != ed25519.PrivateKeySize {
		return Artifact{}, errors.New("invalid signing key")
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	manifest, err := z.Create("manifest.json")
	if err != nil {
		return Artifact{}, err
	}
	if err = json.NewEncoder(manifest).Encode(m); err != nil {
		return Artifact{}, err
	}
	entry, err := z.Create(m.Entrypoint)
	if err != nil {
		return Artifact{}, err
	}
	if _, err = entry.Write(content); err != nil {
		return Artifact{}, err
	}
	if err = z.Close(); err != nil {
		return Artifact{}, err
	}
	a := Artifact{Descriptor: Descriptor{id, m, fmt.Sprintf("%x", sha256.Sum256(buf.Bytes())), buf.Len(), "demo"}, ZIP: buf.Bytes()}
	data, err := json.Marshal(a.Descriptor)
	if err != nil {
		return Artifact{}, err
	}
	a.Signature = ed25519.Sign(key, data)
	return a, a.Verify(key.Public().(ed25519.PublicKey))
}
func (a Artifact) Verify(key ed25519.PublicKey) error {
	d := a.Descriptor
	m := d.Manifest
	if !valid(d.ArtifactID) || !valid(m.ID) || !valid(m.Version) || d.KeyID != "demo" || m.Runtime != "process" || m.Protocol != 1 {
		return errors.New("invalid descriptor")
	}
	if m.IPC != "" && m.IPC != "tcp" && m.IPC != "native" {
		return errors.New("unsupported IPC transport")
	}
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	if len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, data, a.Signature) {
		return errors.New("bad signature")
	}
	if len(a.ZIP) > MaxPackage || len(a.ZIP) != d.Size || fmt.Sprintf("%x", sha256.Sum256(a.ZIP)) != d.SHA256 {
		return errors.New("bad package digest or size")
	}
	return nil
}
func safePath(path string) bool {
	if !fs.ValidPath(path) || path == "." {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if !valid(segment) {
			return false
		}
	}
	return true
}
func (a Artifact) Install(root string, key ed25519.PublicKey, d Desired) (packageFiles, error) {
	if err := a.Verify(key); err != nil {
		return packageFiles{}, err
	}
	m := a.Descriptor.Manifest
	if a.Descriptor.ArtifactID != d.Artifact || m.ID != d.ID || m.Version != d.Version || m.OS != runtime.GOOS || m.Arch != runtime.GOARCH || !safePath(m.Entrypoint) {
		return packageFiles{}, errors.New("manifest does not match desired component or platform")
	}
	dest := filepath.Join(root, m.ID, a.Descriptor.SHA256)
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return packageFiles{}, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(dest), ".stage-")
	if err != nil {
		return packageFiles{}, err
	}
	defer os.RemoveAll(stage)
	reader, err := zip.NewReader(bytes.NewReader(a.ZIP), int64(len(a.ZIP)))
	if err != nil {
		return packageFiles{}, err
	}
	remaining := uint64(MaxPackage)
	seen := map[string][32]byte{}
	for _, f := range reader.File {
		_, duplicate := seen[f.Name]
		if !safePath(f.Name) || duplicate || !f.Mode().IsRegular() || f.UncompressedSize64 > remaining || len(seen) >= 128 {
			return packageFiles{}, errors.New("unsafe ZIP entry")
		}

		remaining -= f.UncompressedSize64
		rc, err := f.Open()
		if err != nil {
			return packageFiles{}, err
		}
		data, err := io.ReadAll(io.LimitReader(rc, int64(f.UncompressedSize64)+1))
		rc.Close()
		if err != nil || uint64(len(data)) != f.UncompressedSize64 {
			return packageFiles{}, errors.New("invalid ZIP entry")
		}
		seen[f.Name] = sha256.Sum256(data)
		if f.Name == "manifest.json" {
			var inside Manifest
			if json.Unmarshal(data, &inside) != nil || inside != m {
				return packageFiles{}, errors.New("manifest mismatch")
			}
		}
		path := filepath.Join(stage, filepath.FromSlash(f.Name))
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return packageFiles{}, err
		}
		if err = os.WriteFile(path, data, 0500); err != nil {
			return packageFiles{}, err
		}
	}
	_, hasEntry := seen[m.Entrypoint]
	_, hasManifest := seen["manifest.json"]
	if !hasEntry || !hasManifest || m.Entrypoint == "manifest.json" {
		return packageFiles{}, errors.New("missing executable or manifest")
	}
	if _, err := os.Stat(dest); errors.Is(err, os.ErrNotExist) {
		if err = os.Rename(stage, dest); err != nil {
			return packageFiles{}, err
		}
	} else if err != nil {
		return packageFiles{}, err
	}
	files := packageFiles{Dir: dest, Entrypoint: m.Entrypoint, Digests: seen}
	if err := files.verify(); err != nil {
		return packageFiles{}, err
	}
	return files, nil
}

type packageFiles struct {
	Dir        string
	Entrypoint string
	Digests    map[string][32]byte
}

func (p packageFiles) executable() string {
	return filepath.Join(p.Dir, filepath.FromSlash(p.Entrypoint))
}

// The expected digests come from the signed ZIP, not from the installed files.
func (p packageFiles) verify() error {
	info, err := os.Lstat(p.Dir)
	if err != nil {
		return fmt.Errorf("verify install directory: %w", err)
	}
	if !info.IsDir() || !safePath(p.Entrypoint) || len(p.Digests) == 0 {
		return errors.New("invalid installed package metadata")
	}
	root, err := os.OpenRoot(p.Dir)
	if err != nil {
		return err
	}
	defer root.Close()
	seen := 0
	err = fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		digest, ok := p.Digests[path]
		if !ok || !entry.Type().IsRegular() {
			return fmt.Errorf("unexpected installed file: %s", path)
		}
		f, err := root.Open(filepath.FromSlash(path))
		if err != nil {
			return err
		}
		hash := sha256.New()
		count, readErr := io.Copy(hash, io.LimitReader(f, MaxPackage+1))
		closeErr := f.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if count > MaxPackage || !bytes.Equal(hash.Sum(nil), digest[:]) {
			return fmt.Errorf("installed file digest mismatch: %s", path)
		}
		seen++
		return nil
	})
	if err != nil {
		return err
	}
	if seen != len(p.Digests) {
		return errors.New("installed package is missing files")
	}
	return nil
}
