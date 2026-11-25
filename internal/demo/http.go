package demo

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
)

const maxJSON = 46 << 20

func Decode(r io.Reader, target any) error {
	d := json.NewDecoder(io.LimitReader(r, maxJSON+1))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("unexpected data after JSON")
	}
	return nil
}

func TLS(dir, role string) (*tls.Config, error) {
	ca, err := os.ReadFile(filepath.Join(dir, "ca.pem"))
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, errors.New("invalid CA")
	}
	pair, err := tls.LoadX509KeyPair(filepath.Join(dir, role+".pem"), filepath.Join(dir, role+".key"))
	if err != nil {
		return nil, err
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pair}, RootCAs: pool, ClientCAs: pool}
	if role == "server" {
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return cfg, nil
}

type Client struct {
	URL  string
	HTTP *http.Client
}

func NewClient(url, dir, role string) (Client, error) {
	if len(url) < 8 || url[:8] != "https://" {
		return Client{}, errors.New("HTTPS required")
	}
	cfg, err := TLS(dir, role)
	if err != nil {
		return Client{}, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = cfg
	transport.ForceAttemptHTTP2 = true
	return Client{url, &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect rejected") }}}, nil
}
func (c Client) Request(method, path string, input, output any) error {
	return c.request(context.Background(), method, path, input, output)
}

func (c Client) request(ctx context.Context, method, path string, input, output any) error {
	var data []byte
	if input != nil {
		var err error
		data, err = json.Marshal(input)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.URL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("%s: %s %s", path, resp.Status, body)
	}
	if output != nil {
		return Decode(resp.Body, output)
	}
	return nil
}
func Server(store Store, registry string, key ed25519.PublicKey, enrollment string) http.Handler {
	mux := http.NewServeMux()
	var lock sync.Mutex
	type handler func(http.ResponseWriter, *http.Request, string) error
	add := func(route, role string, h handler) {
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 {
				http.Error(w, "client certificate required", 401)
				return
			}
			cert := r.TLS.VerifiedChains[0][0]
			id := cert.Subject.CommonName
			if len(cert.Subject.OrganizationalUnit) != 1 || cert.Subject.OrganizationalUnit[0] != role || !valid(id) {
				http.Error(w, "wrong certificate role", 403)
				return
			}
			if r.URL.Path == "/v1/agents/enroll" || role == "admin" && r.Method != http.MethodGet {
				lock.Lock()
				defer lock.Unlock()
			}
			if role == "agent" && r.URL.Path != "/v1/agents/enroll" {
				var enrolled string
				if err := store.Get("enrolled_certificate", &enrolled); err != nil || enrolled != certificateID(r) {
					http.Error(w, "enrollment required", 403)
					return
				}
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxJSON)
			if err := h(w, r, id); err != nil {
				http.Error(w, err.Error(), 400)
			}
		})
	}
	add("POST /v1/agents/enroll", "agent", func(w http.ResponseWriter, r *http.Request, id string) error {
		var enrolled string
		if err := store.Get("enrolled_certificate", &enrolled); err != nil {
			return err
		}
		fingerprint := certificateID(r)
		if enrolled == fingerprint {
			return nil
		}
		if enrolled != "" {
			return errors.New("enrollment credential already used")
		}
		var token string
		if err := Decode(r.Body, &token); err != nil {
			return err
		}
		if enrollment == "" || subtle.ConstantTimeCompare([]byte(token), []byte(enrollment)) != 1 {
			return errors.New("invalid enrollment token")
		}
		return store.Put("enrolled_certificate", fingerprint)
	})
	add("POST /v1/agents/checkin", "agent", func(w http.ResponseWriter, r *http.Request, id string) error {
		var report Report
		if err := Decode(r.Body, &report); err != nil {
			return err
		}
		report.CheckinAt = time.Now().UTC()
		if err := store.Put("report/"+id, report); err != nil {
			return err
		}
		var policy Policy
		if err := store.Get("policy/"+id, &policy); err != nil {
			return err
		}
		return json.NewEncoder(w).Encode(policy)
	})
	add("POST /v1/agents/events", "agent", func(w http.ResponseWriter, r *http.Request, id string) error {
		return receiveEvents(w, r, store, id)
	})
	add("GET /v1/admin/agents/{id}", "admin", func(w http.ResponseWriter, r *http.Request, _ string) error {
		var report *Report
		if err := store.Get("report/"+r.PathValue("id"), &report); err != nil {
			return err
		}
		if report == nil {
			return errors.New("agent has not checked in")
		}
		return json.NewEncoder(w).Encode(report)
	})
	add("PUT /v1/admin/agents/{id}/policy", "admin", func(w http.ResponseWriter, r *http.Request, _ string) error {
		id := r.PathValue("id")
		if !valid(id) {
			return errors.New("invalid agent ID")
		}
		var p, old Policy
		if err := Decode(r.Body, &p); err != nil {
			return err
		}
		if err := p.Validate(); err != nil {
			return err
		}
		if err := store.Get("policy/"+id, &old); err != nil {
			return err
		}
		if p.Revision < old.Revision || p.Revision == old.Revision && !reflect.DeepEqual(old, p) {
			return errors.New("stale or conflicting revision")
		}
		return store.Put("policy/"+id, p)
	})
	add("POST /v1/admin/artifacts", "admin", func(w http.ResponseWriter, r *http.Request, _ string) error {
		var a Artifact
		if err := Decode(r.Body, &a); err != nil {
			return err
		}
		if err := a.Verify(key); err != nil {
			return err
		}
		path := filepath.Join(registry, a.Descriptor.ArtifactID+".json")
		data, err := json.Marshal(a)
		if err != nil {
			return err
		}
		old, err := os.ReadFile(path)
		if err == nil {
			if bytes.Equal(old, data) {
				return nil
			}
			return errors.New("artifact ID already exists")
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		f, err := os.CreateTemp(registry, ".upload-")
		if err != nil {
			return err
		}
		defer os.Remove(f.Name())
		_, err = f.Write(data)
		if err == nil {
			err = f.Sync()
		}
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		return os.Rename(f.Name(), path)
	})
	add("GET /v1/artifacts/{id}", "agent", func(w http.ResponseWriter, r *http.Request, agent string) error {
		id := r.PathValue("id")
		if !valid(id) {
			return errors.New("invalid artifact ID")
		}
		var p Policy
		if err := store.Get("policy/"+agent, &p); err != nil {
			return err
		}
		for _, d := range p.Components {
			if d.State != "absent" && d.Artifact == id {
				http.ServeFile(w, r, filepath.Join(registry, id+".json"))
				return nil
			}
		}
		return errors.New("artifact not assigned")
	})
	return mux
}

func certificateID(r *http.Request) string {
	return fmt.Sprintf("%x", sha256.Sum256(r.TLS.VerifiedChains[0][0].Raw))
}
