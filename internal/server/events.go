package server

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// hub is a tiny in-process server-sent-events fan-out. Write paths call
// broadcast() after a run changes state; every connected SPA tab receives a
// nudge and re-fetches the resource it's showing. This replaces fixed-interval
// polling: idle systems push nothing, busy ones push at most one event per
// state change (the client debounces refetches).
//
// Broadcast is non-blocking — a slow/stuck subscriber is skipped rather than
// stalling a request that holds writeMu. Each subscriber channel is buffered
// so a single coalescable event isn't lost between reads.
type hub struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

func newHub() *hub {
	return &hub{subs: map[chan []byte]struct{}{}}
}

func (h *hub) subscribe() chan []byte {
	ch := make(chan []byte, 8)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *hub) unsubscribe(ch chan []byte) {
	h.mu.Lock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
	h.mu.Unlock()
}

func (h *hub) broadcast(msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- msg:
		default:
			// Subscriber is behind; drop. Events are coalescable nudges, so a
			// missed one only delays a refetch until the next event.
		}
	}
}

// notify publishes a coarse change event. kind is an opaque hint ("run",
// "runs", "claim") — the SPA refetches regardless, so the exact value is only
// for debugging in the network panel.
func (s *Deps) notify(kind string) {
	if s.hub == nil {
		return
	}
	s.hub.broadcast([]byte(fmt.Sprintf(`{"kind":%q}`, kind)))
}

// handleEvents is the GET /api/events SSE stream. It lives under the basicAuth
// group; since EventSource can't set an Authorization header, basicAuth also
// accepts the credentials via the ?auth= query param for this endpoint.
func (s *Deps) handleEvents(w http.ResponseWriter, req *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx proxy buffering

	ch := s.hub.subscribe()
	defer s.hub.unsubscribe(ch)

	// Open the stream so the browser fires onopen and the client stops any
	// fallback polling.
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	ctx := req.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: change\ndata: %s\n\n", msg)
			flusher.Flush()
		case <-keepalive.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
