package main

import (
	"log"
	"net/http"
)

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
	ew := &eventWorker{make(chan []byte, 48)}
	register <- ew
	var err error
	write := func(buf []byte) {
		if err == nil {
			_, err = w.Write(buf)
		}
	}

	for {
		select {
		case msg := <-ew.events:
			write([]byte("data: "))
			write(msg)
			write([]byte("\n\n"))
			flusher.Flush()
			if err != nil {
				unregister <- ew
				return
			}

		case <-closenotify:
			unregister <- ew
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
			buf, err := ev.eventString()
			if err != nil {
				log.Printf("sse: marshalling event: %s\n", err)
				continue
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
