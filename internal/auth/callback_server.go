package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/proofboard/proofboard/internal/model"
)

type CallbackServer struct {
	Port int
}

func (s CallbackServer) Wait(ctx context.Context) (model.Credentials, error) {
	if err := ctx.Err(); err != nil {
		return model.Credentials{}, fmt.Errorf("wait for auth callback: %w", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(s.Port))
	if err != nil {
		return model.Credentials{}, fmt.Errorf("listen for auth callback: %w", err)
	}
	defer listener.Close()

	result := make(chan model.Credentials, 1)
	errs := make(chan error, 1)
	mux := http.NewServeMux()
	server := &http.Server{Handler: mux}
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		token := query.Get("token")
		if token == "" {
			http.Error(w, "missing token", http.StatusBadRequest)
			errs <- fmt.Errorf("auth callback missing token")
			return
		}
		credentials := model.Credentials{
			Token:        token,
			Username:     query.Get("username"),
			RefreshToken: query.Get("refreshToken"),
			EmailHash:    query.Get("emailHash"),
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Proofboard CLI authenticated. You can close this tab."))
		result <- credentials
	})
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errs <- err
		}
	}()
	defer server.Shutdown(context.Background())

	select {
	case credentials := <-result:
		return credentials, nil
	case err := <-errs:
		return model.Credentials{}, fmt.Errorf("auth callback failed: %w", err)
	case <-ctx.Done():
		return model.Credentials{}, fmt.Errorf("auth callback cancelled: %w", ctx.Err())
	}
}
