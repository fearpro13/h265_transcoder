package h265_transcoder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

type OnCreate func(id string, source string) (Source, error)
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
	ctxF    context.CancelFunc
	ctx     context.Context
	Done    <-chan struct{}
}

func NewControlServer(pCtx context.Context, httpPort uint16) *ControlServer {
	handler := &http.ServeMux{}
	panicHandler := &http.ServeMux{}

	server := &http.Server{
		Addr:                         fmt.Sprintf(":%d", httpPort),
		Handler:                      panicHandler,
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

	ctx, ctxF := context.WithCancel(pCtx)
	controlServer := &ControlServer{hs: server, running: atomic.Bool{}, Done: ctx.Done(), ctx: ctx, ctxF: ctxF}

	panicHandler.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			r := recover()
			if r != nil {
				var err error
				switch t := r.(type) {
				case string:
					err = errors.New(t)
				case error:
					err = t
				default:
					err = errors.New("unknown error")
				}

				log.Printf("http_control: panic: %s, stopping\n", err.Error())
				controlServer.ctxF()
			}
		}()

		handler.ServeHTTP(writer, request)
	})

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

		encoder := json.NewEncoder(w)

		var addedSource Source
		addedSource, err = controlServer.OnCreate(id, source)
		if err != nil {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)

			_ = encoder.Encode(map[string]string{
				"source": addedSource.to.String(),
			})

			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = encoder.Encode(map[string]string{
			"source": addedSource.to.String(),
		})

	})

	handler.HandleFunc("POST /{id}/stop", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		err := controlServer.OnStop(id)
		if err != nil {
			w.Header().Add("Content-Type", "application/json")
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
		s.ctxF()
	}()

	return nil
}

func (s *ControlServer) Stop() error {
	if !s.running.Load() {
		return errors.New("not running")
	}

	s.running.Store(false)
	s.ctxF()

	return s.hs.Shutdown(context.Background())
}
