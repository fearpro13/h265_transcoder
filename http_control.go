package h265_transcoder

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync/atomic"
)

type OnCreate func(id string, source string) error
type OnStop func(id string) error
type OnStatus func(id string) map[string]any
type OnStatusAll func() map[string]any

type ControlServer struct {
	hs *http.Server
	OnCreate
	OnStop
	OnStatus
	OnStatusAll
	running atomic.Bool
}

func NewControlServer(addr string) *ControlServer {
	handler := &http.ServeMux{}
	server := &http.Server{
		Addr:                         addr,
		Handler:                      handler,
		DisableGeneralOptionsHandler: false,
		TLSConfig:                    nil,
		ReadTimeout:                  0,
		ReadHeaderTimeout:            0,
		WriteTimeout:                 0,
		IdleTimeout:                  0,
		MaxHeaderBytes:               0,
		TLSNextProto:                 nil,
		ConnState:                    nil,
		ErrorLog:                     nil,
		BaseContext:                  nil,
		ConnContext:                  nil,
	}

	controlServer := &ControlServer{hs: server, running: atomic.Bool{}}

	handler.HandleFunc("POST /create", func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)

		data := map[string]any{}

		err := decoder.Decode(&data)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		id, ok := data["id"].(string)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		source, ok := data["source"].(string)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		_ = r.Body.Close()

		err = controlServer.OnCreate(id, source)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)

			encoder := json.NewEncoder(w)

			_ = encoder.Encode(map[string]string{
				"message": err.Error(),
			})

			return
		}

		w.WriteHeader(http.StatusOK)
	})

	handler.HandleFunc("POST /{id}/stop", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		err := controlServer.OnStop(id)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)

			encoder := json.NewEncoder(w)

			_ = encoder.Encode(map[string]string{
				"message": err.Error(),
			})

			return
		}

		w.WriteHeader(http.StatusOK)
	})

	handler.HandleFunc("GET /{id}/status", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		encoder := json.NewEncoder(w)

		status := controlServer.OnStatus(id)
		if status == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = encoder.Encode(status)
	})

	handler.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		encoder := json.NewEncoder(w)

		statusAll := controlServer.OnStatusAll()
		if statusAll == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = encoder.Encode(statusAll)
	})

	return controlServer
}

func (s *ControlServer) Start() error {
	if s.running.Load() {
		return errors.New("already started")
	}

	s.running.Store(true)
	go func() {
		log.Printf("Control server[HTTP] listening on %s\n", s.hs.Addr)
		err := s.hs.ListenAndServe()
		if err != nil {
			_ = s.Stop()
		}
	}()

	return nil
}

func (s *ControlServer) Stop() error {
	if !s.running.Load() {
		return errors.New("not running")
	}

	s.running.Store(false)

	return s.hs.Shutdown(context.Background())
}
