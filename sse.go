package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// Ding offers real-time updates to repositories and builds (including command output) through Server-Sent Events (SSE).
// You'll receive the following events with a HTTP GET request to `/events`, encoded as JSON:
// - `repo`, repository was updated or created
// - `removeRepo`, repository was removed
// - `build`, build was updated or created
// - `removeBuild`, build was removed
// - `output`, new lines of output from a command for an active build
//
// These types are described below, with an _event_-prefix. E.g. type _eventRepo_ describes the `repo` event.
type SSE struct {
}

// No-op. This function only serves to include documentation for the server-sent event types.
func (SSE) ExampleSSE() (repo eventRepo, removeRepo eventRemoveRepo, build eventBuild, removeBuild eventRemoveBuild, output eventOutput) {
	return
}

type eventWorker struct {
	events chan []byte
}

func serveEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Println("internal error: ResponseWriter not a http.Flusher")
		http.Error(w, "internal error", 500)
		return
	}
	closenotifier, ok := w.(http.CloseNotifier)
	if !ok {
		log.Println("internal error: ResponseWriter not a http.CloseNotifier")
		http.Error(w, "internal error", 500)
		return
	}
	closenotify := closenotifier.CloseNotify()
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	_, err := w.Write([]byte(": keepalive\n\n"))
	if err != nil {
		return
	}
	flusher.Flush()

	ew := &eventWorker{make(chan []byte, 48)}
	register <- ew
	defer func() {
		unregister <- ew
	}()

	for {
		select {
		case msg := <-ew.events:
			_, err = w.Write(msg)
			flusher.Flush()
			if err != nil {
				return
			}

		case <-closenotify:
			return
		}
	}
}

var (
	register   chan *eventWorker
	unregister chan *eventWorker
	events     chan eventStringer
)

func init() {
	register = make(chan *eventWorker, 1)
	unregister = make(chan *eventWorker, 0)
	events = make(chan eventStringer, 10)

	go func() {
		for {
			time.Sleep(120 * time.Second)
			events <- nil
		}
	}()
}

func eventMux() {
	workers := []*eventWorker{}
	for {
		select {
		case ew := <-register:
			workers = append(workers, ew)
		case ew := <-unregister:
			nworkers := []*eventWorker{}
			for _, x := range workers {
				if x != ew {
					nworkers = append(nworkers, x)
				}
			}
		case ev := <-events:
			var buf []byte
			if ev == nil {
				buf = []byte(": keepalive\n\n")
			} else {
				event, evbuf, err := ev.eventString()
				if err != nil {
					log.Printf("sse: marshalling event: %s\n", err)
					continue
				}
				buf = []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, evbuf))
			}
			for _, w := range workers {
				select {
				case w.events <- buf:
				default:
					// log.Println("sse: dropping event, client cannot keep up...")
				}
			}
		}
	}
}
