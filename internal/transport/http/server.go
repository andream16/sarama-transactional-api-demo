package http

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
)

type (
	RegistrationAwareHandler interface {
		Path() string
		Register(mux *http.ServeMux)
	}

	Server struct {
		srv *http.Server
	}
)

func NewServer(listenAddr string, handlers ...RegistrationAwareHandler) Server {
	mux := http.NewServeMux()

	for _, h := range handlers {
		h.Register(mux)
	}

	return Server{
		srv: &http.Server{
			Addr:         listenAddr,
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 1 * time.Second,
			IdleTimeout:  60 * time.Second,
		}}
}

func (s Server) ListenAndServe() error {
	log.Printf("server started...\n")
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("unexpected listen & serve error: %w", err)
	}
	return nil
}

func (s Server) Shutdown(ctx context.Context) error {
	if err := s.srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		if err := s.srv.Close(); err != nil {
			return fmt.Errorf("unexpected shutdown error: %w", err)
		}
	}
	return nil
}
