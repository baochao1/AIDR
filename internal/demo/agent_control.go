package demo

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

const controlInterval = 600 * time.Millisecond

func checkin(ctx context.Context, c Client, token string, latest *atomic.Pointer[Report], policies chan<- Policy) {
	enrolled := false
	connected := false
	ticker := time.NewTicker(controlInterval)
	defer ticker.Stop()
	for ctx.Err() == nil {
		if !enrolled {
			if err := c.request(ctx, http.MethodPost, "/v1/agents/enroll", token, nil); err != nil {
				if ctx.Err() == nil {
					log.Print(err)
				}
			} else {
				enrolled = true
				log.Print("management enrollment accepted")
			}
		}
		if enrolled {
			var p Policy
			if err := c.request(ctx, http.MethodPost, "/v1/agents/checkin", latest.Load(), &p); err != nil {
				if ctx.Err() == nil {
					log.Print(err)
				}
				connected = false
			} else {
				if !connected {
					log.Print("management check-in accepted")
					connected = true
				}
				if p.Revision != 0 {
					select {
					case policies <- p:
					case <-ctx.Done():
						return
					}
				}
			}
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

var errDownloading = errors.New("downloading artifact")

type downloadResult struct {
	artifact Artifact
	err      error
}
type download struct {
	id     string
	cancel context.CancelFunc
	done   chan downloadResult
}

func (a *coordinator) downloadArtifact(ctx context.Context, d Desired) (Artifact, error) {
	current := a.downloads[d.ID]
	if current != nil {
		if current.id == d.Artifact {
			select {
			case result := <-current.done:
				current.cancel()
				delete(a.downloads, d.ID)
				return result.artifact, result.err
			default:
				return Artifact{}, errDownloading
			}
		}
		current.cancel()
	}
	requestCtx, cancel := context.WithCancel(ctx)
	current = &download{id: d.Artifact, cancel: cancel, done: make(chan downloadResult, 1)}
	a.downloads[d.ID] = current
	go func() {
		var artifact Artifact
		err := a.client.request(requestCtx, http.MethodGet, "/v1/artifacts/"+d.Artifact, nil, &artifact)
		current.done <- downloadResult{artifact, err}
	}()
	return Artifact{}, errDownloading
}
