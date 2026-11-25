package demo

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	pb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"supervisor-demo/internal/component"
)

type installed struct {
	Desired  Desired
	Path     string
	Rejected uint64
	Error    string
	IPC      string
	Files    *packageFiles
}
type agentState struct {
	Policy  Policy
	Active  map[string]installed
	Applied uint64
}
type child struct {
	cmd            *exec.Cmd
	conn           *grpc.ClientConn
	token          string
	done           chan struct{}
	parent         io.WriteCloser
	cleanup        func(bool)
	cancelEvents   context.CancelFunc
	eventsDone     chan struct{}
	removeEndpoint func()
	data           atomic.Pointer[DataStatus]
	probe          atomic.Pointer[probeResult]
	cancelProbe    context.CancelFunc
}

func (c *child) stop() {
	if c.cancelProbe != nil {
		c.cancelProbe()
	}
	if c.cancelEvents != nil {
		c.cancelEvents()
	}
	c.conn.Close()
	if c.parent != nil {
		_ = c.parent.Close()
		select {
		case <-c.done:
		case <-time.After(350 * time.Millisecond):
		}
	}
	select {
	case <-c.done:
		c.cleanup(false)
	default:
		c.cleanup(true)
	}
	<-c.done
	if c.eventsDone != nil {
		<-c.eventsDone
	}
	c.removeEndpoint()
}
func start(ctx context.Context, i installed, dir string) (*child, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(nonce)
	native := i.IPC == "native"
	transport := "tcp"
	var endpoint string
	removeEndpoint := func() {}
	if native {
		transport = component.NativeTransport
		digest := sha256.Sum256(nonce)
		address, cleanup, err := component.NativeEndpoint(hex.EncodeToString(digest[:16]))
		if err != nil {
			return nil, err
		}
		endpoint, removeEndpoint = address, cleanup
	} else {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		endpoint = listener.Addr().String()
		listener.Close()
	}
	started := false
	defer func() {
		if !started {
			removeEndpoint()
		}
	}()
	cert, key, err := instanceCertificate()
	if err != nil {
		return nil, err
	}
	conn, err := component.DialPinned(endpoint, cert, native)
	if err != nil {
		return nil, err
	}
	c := &child{cmd: exec.Command(i.Path), conn: conn, token: token, done: make(chan struct{}), removeEndpoint: removeEndpoint}
	c.cmd.Dir = dir
	for _, name := range []string{"PATH", "SystemRoot", "WINDIR", "TEMP", "TMP", "TMPDIR", "LANG"} {
		if value, ok := os.LookupEnv(name); ok {
			c.cmd.Env = append(c.cmd.Env, name+"="+value)
		}
	}
	c.cmd.Env = append(c.cmd.Env, "DEMO_ENDPOINT="+endpoint, "DEMO_TOKEN="+token, "DEMO_VERSION="+i.Desired.Version,
		"DEMO_PARENT_PIPE=1", "DEMO_IPC_CERT="+string(cert), "DEMO_IPC_KEY="+string(key), "DEMO_IPC_TRANSPORT="+transport)
	c.cmd.Stdout = os.Stdout
	c.cmd.Stderr = os.Stderr
	c.parent, err = c.cmd.StdinPipe()
	if err != nil {
		conn.Close()
		return nil, err
	}
	c.cleanup, err = startProcess(c.cmd)
	if err != nil {
		c.parent.Close()
		conn.Close()
		return nil, err
	}
	go func() { _ = c.cmd.Wait(); close(c.done) }()
	started = true
	deadline, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	passed := 0
	for deadline.Err() == nil {
		select {
		case <-c.done:
			c.stop()
			return nil, errors.New("component exited during startup")
		default:
		}
		if c.health(deadline) == nil {
			passed++
			if passed == 3 {
				return c, nil
			}
		} else {
			passed = 0
		}
		select {
		case <-deadline.Done():
		case <-time.After(120 * time.Millisecond):
		}
	}
	c.stop()
	return nil, errors.New("startup health checks failed")
}
func (c *child) health(ctx context.Context) error {
	select {
	case <-c.done:
		return errors.New("component exited")
	default:
	}
	return component.Probe(ctx, pb.NewHealthClient(c.conn), c.token)
}

type coordinator struct {
	store     Store
	client    Client
	key       ed25519.PublicKey
	root      string
	state     agentState
	processes map[string]*child
	failures  map[string][]time.Time
	lastFault map[string]string
	next      map[string]time.Time
	events    bool
	downloads map[string]*download
}

func RunAgent(ctx context.Context, s Store, c Client, key ed25519.PublicKey, root, token string, events bool) error {
	a := coordinator{
		store: s, client: c, key: key, root: root, events: events,
		processes: map[string]*child{}, failures: map[string][]time.Time{}, lastFault: map[string]string{},
		next: map[string]time.Time{}, downloads: map[string]*download{},
	}
	if err := s.Get("agent", &a.state); err != nil {
		return err
	}
	if a.state.Active == nil {
		a.state.Active = map[string]installed{}
	}
	defer func() {
		for id := range a.processes {
			a.stop(id)
		}
	}()
	if events {
		uploadCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() {
			defer close(done)
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-uploadCtx.Done():
					return
				case <-ticker.C:
					if err := s.flushEvents(uploadCtx, c); err != nil && uploadCtx.Err() == nil {
						log.Printf("upload events: %v", err)
					}
				}
			}
		}()
		defer func() { cancel(); <-done }()
	}
	controlCtx, cancelControl := context.WithCancel(ctx)
	controlDone := make(chan struct{})
	policies := make(chan Policy, 1)
	var latest atomic.Pointer[Report]
	latest.Store(&Report{Components: map[string]Status{}})
	go func() {
		defer close(controlDone)
		checkin(controlCtx, c, token, &latest, policies)
	}()
	defer func() {
		cancelControl()
		<-controlDone
		for _, download := range a.downloads {
			download.cancel()
		}
	}()
	ticker := time.NewTicker(controlInterval)
	defer ticker.Stop()
	for ctx.Err() == nil {
		report, err := a.reconcile(ctx)
		if err != nil {
			return err
		}
		latest.Store(&report)
		select {
		case <-ctx.Done():
		case <-ticker.C:
		case incoming := <-policies:
			if err := a.accept(incoming); err != nil {
				log.Print(err)
			}
		}
	}
	return nil
}
func (a *coordinator) accept(p Policy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	old := a.state.Policy
	if p.Revision < old.Revision || p.Revision == old.Revision && !reflect.DeepEqual(p, old) {
		return errors.New("stale or conflicting policy")
	}
	if p.Revision == old.Revision {
		return nil
	}
	updated := a.state
	updated.Policy = p
	if err := a.store.Put("agent", updated); err != nil {
		return err
	}
	a.state.Policy = p
	wanted := make(map[string]string, len(p.Components))
	for _, d := range p.Components {
		wanted[d.ID] = d.Artifact
		delete(a.failures, d.ID)
		delete(a.lastFault, d.ID)
		delete(a.next, d.ID)
	}
	for id, request := range a.downloads {
		if wanted[id] != request.id {
			request.cancel()
			delete(a.downloads, id)
		}
	}
	return nil
}
func (a *coordinator) stop(id string) {
	if p := a.processes[id]; p != nil {
		p.stop()
		delete(a.processes, id)
	}
}
func (a *coordinator) activate(ctx context.Context, i installed) error {
	id := i.Desired.ID
	if i.Desired.State != "running" {
		return nil
	}
	if i.Files == nil || i.Files.executable() != i.Path {
		return errors.New("missing verified installed package metadata")
	}
	if err := i.Files.verify(); err != nil {
		return fmt.Errorf("component integrity check: %w", err)
	}
	dir := filepath.Join(a.root, "data", id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	p, err := start(ctx, i, dir)
	if err == nil {
		a.processes[id] = p
		probeCtx, cancel := context.WithCancel(ctx)
		p.cancelProbe = cancel
		go p.monitorHealth(probeCtx)
		if a.events {
			collectCtx, cancel := context.WithCancel(ctx)
			p.cancelEvents = cancel
			p.eventsDone = make(chan struct{})
			go func() {
				defer close(p.eventsDone)
				a.collect(collectCtx, id, p)
			}()
		}
	}
	return err
}

func (a *coordinator) collect(ctx context.Context, id string, p *child) {
	digest := sha256.Sum256([]byte(p.token))
	instance := hex.EncodeToString(digest[:16])
	var sequence uint64
	data := DataStatus{State: "connecting"}
	publish := func() { snapshot := data; p.data.Store(&snapshot) }
	publish()
	delay := 250 * time.Millisecond
	for ctx.Err() == nil {
		err := component.Collect(ctx, p.conn, p.token, func(payload string) (string, error) {
			if len(payload) > 1024 {
				return "", errors.New("event payload exceeds limit")
			}
			sequence++
			eventID := fmt.Sprintf("%s-%x", instance, sequence)
			observed := time.Now().UTC()
			event := Event{eventID, id, instance, 1, observed, payload, sequence}
			for {
				err := a.store.enqueue(event)
				if err == nil {
					data = DataStatus{State: "receiving", LastEventAt: &observed}
					publish()
					delay = 250 * time.Millisecond
					return eventID, nil
				}
				if !errors.Is(err, errQueueFull) {
					return "", err
				}
				data.State = "backpressure"
				publish()
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(time.Second):
				}
			}
		})
		if ctx.Err() != nil {
			return
		}
		if status.Code(err) == codes.Unimplemented {
			data.State = "unsupported"
			publish()
			return
		}
		data.State = "reconnecting"
		if err != nil {
			data.Error = err.Error()
		}
		publish()
		log.Printf("collect events for %s: %v; retrying in %s", id, err, delay)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(2*delay, 4*time.Second)
	}
}

type probeResult struct {
	failures int
	error    string
}

func (p *child) monitorHealth(ctx context.Context) {
	ticker := time.NewTicker(controlInterval)
	defer ticker.Stop()
	var result probeResult
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.done:
			return
		case <-ticker.C:
		}
		if err := p.health(ctx); err != nil {
			result.failures++
			result.error = err.Error()
		} else {
			result = probeResult{}
		}
		snapshot := result
		p.probe.Store(&snapshot)
	}
}

const restartWindow = time.Minute
const restartLimit = 5

func (a *coordinator) restartFailures(id string, now time.Time) []time.Time {
	failures := a.failures[id]
	for len(failures) > 0 && now.Sub(failures[0]) >= restartWindow {
		failures = failures[1:]
	}
	a.failures[id] = failures
	return failures
}
func (a *coordinator) recordFault(id string, err string, now time.Time) {
	failures := append(a.restartFailures(id, now), now)
	a.failures[id] = failures
	a.lastFault[id] = err
	a.next[id] = now.Add(time.Second << min(len(failures)-1, 4))
}

func (a *coordinator) change(ctx context.Context, d Desired) error {
	id := d.ID
	old := a.state.Active[id]
	if d.State == "absent" {
		a.stop(id)
		if request := a.downloads[id]; request != nil {
			request.cancel()
			delete(a.downloads, id)
		}
		old.Desired.State = "stopped"
		a.state.Active[id] = old
		if err := os.RemoveAll(filepath.Join(a.root, "packages", id)); err != nil {
			return err
		}
		delete(a.state.Active, id)
		return nil
	}
	candidate := installed{Desired: d, Path: old.Path, IPC: old.IPC, Files: old.Files}
	if old.Desired.Artifact != d.Artifact || old.Desired.Version != d.Version || old.Files == nil {
		artifact, err := a.downloadArtifact(ctx, d)
		if err != nil {
			return err
		}
		files, err := artifact.Install(filepath.Join(a.root, "packages"), a.key, d)
		if err != nil {
			return err
		}
		candidate.Path = files.executable()
		candidate.Files = &files
		candidate.IPC = artifact.Descriptor.Manifest.IPC
	}
	a.stop(id)
	log.Printf("component %s transition %s/%s -> %s/%s", id, old.Desired.Version, old.Desired.State, d.Version, d.State)
	if err := a.activate(ctx, candidate); err != nil {
		if old.Path != "" {
			if rollback := a.activate(ctx, old); rollback != nil {
				err = fmt.Errorf("%w; rollback failed: %v", err, rollback)
			}
		}
		old.Rejected = a.state.Policy.Revision
		old.Error = err.Error()
		a.state.Active[id] = old
		return err
	}
	a.state.Active[id] = candidate
	return nil
}
func (a *coordinator) reconcile(ctx context.Context) (Report, error) {
	report := Report{Received: a.state.Policy.Revision, Applied: a.state.Policy.Revision, Components: map[string]Status{}}
	desired := make(map[string]Desired, len(a.state.Policy.Components))
	for _, d := range a.state.Policy.Components {
		desired[d.ID] = d
		i := a.state.Active[d.ID]
		if d.State == "absent" && i == (installed{}) {
			continue
		}
		if (i.Desired != d || i.Error != "" || i.Files == nil) && i.Rejected != a.state.Policy.Revision {
			err := a.change(ctx, d)
			if errors.Is(err, errDownloading) {
				report.Applied = 0
				continue
			}
			if err != nil {
				i = a.state.Active[d.ID]
				i.Error = err.Error()
				a.state.Active[d.ID] = i
				report.Applied = 0
			}
			if err = a.store.Put("agent", a.state); err != nil {
				return report, err
			}
		}
	}
	for id, i := range a.state.Active {
		st := Status{Version: i.Desired.Version, DesiredVersion: desired[id].Version, State: "stopped", Error: i.Error}
		if i.Path != "" {
			st.IPC = "tcp"
			if i.IPC == "native" {
				st.IPC = component.NativeTransport
			}
		} else {
			st.State = "failed"
		}
		if i.Desired.State == "running" {
			now := time.Now()
			if p := a.processes[id]; p != nil {
				probe := p.probe.Load()
				failure := ""
				select {
				case <-p.done:
					failure = "component exited"
				default:
				}
				if probe != nil && probe.failures >= 3 {
					failure = probe.error
				}
				if failure != "" {
					a.stop(id)
					a.recordFault(id, failure, now)
				} else {
					st.State = "running"
					st.PID = p.cmd.Process.Pid
					if probe != nil && probe.failures > 0 {
						st.State = "degraded"
						st.Error = probe.error
					}
				}
			}
			if a.processes[id] == nil {
				st.State = "backoff"
				st.Error = a.lastFault[id]
				if len(a.restartFailures(id, now)) >= restartLimit {
					st.State = "restart_limited"
				} else if !now.Before(a.next[id]) {
					if err := a.activate(ctx, i); err != nil {
						a.recordFault(id, err.Error(), now)
						st.Error = err.Error()
					} else {
						st.State = "running"
						st.PID = a.processes[id].cmd.Process.Pid
						st.Error = i.Error
					}
				}
			}
			if p := a.processes[id]; p != nil {
				st.Data = p.data.Load()
			}
		}
		report.Components[id] = st
		if st.Error != "" || st.State == "failed" || st.State == "backoff" || st.State == "degraded" || st.State == "restart_limited" {
			report.Applied = 0
		}
	}
	for id := range a.downloads {
		st := report.Components[id]
		if st.State == "" || st.State == "failed" {
			st.State = "installing"
		}
		st.DesiredVersion = desired[id].Version
		report.Components[id] = st
	}
	if report.Applied > a.state.Applied {
		a.state.Applied = report.Applied
		if err := a.store.Put("agent", a.state); err != nil {
			return report, err
		}
	}
	report.Applied = a.state.Applied
	return report, nil
}
