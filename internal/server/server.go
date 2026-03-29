// Package server provides the HTTP server with graceful shutdown support.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Server wraps an HTTP server with graceful shutdown.
type Server struct {
	srv *http.Server
	log *slog.Logger
}

// New creates a new Server listening on addr with the given handler.
func New(addr string, handler http.Handler) *Server {
	return &Server{
		srv: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		log: slog.With("component", "server"),
	}
}

// Start begins serving and blocks until SIGINT/SIGTERM or ctx cancellation.
func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.srv.Addr, err)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		s.log.Info("server started", "addr", s.srv.Addr)
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("serving: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-quit:
		s.log.Info("shutdown signal received")
	case <-ctx.Done():
		s.log.Info("context cancelled")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	s.log.Info("server stopped")
	return nil
}
