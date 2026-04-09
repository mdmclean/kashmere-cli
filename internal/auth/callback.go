// internal/auth/callback.go
package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// WaitForCallback starts a local HTTP server on a random port, returns the
// port and a channel that receives the token when the browser redirects to
// http://localhost:<port>/callback?token=<jwt>.
//
// The server shuts down automatically after receiving a token or after timeout.
func WaitForCallback(timeout time.Duration) (port int, tokenCh <-chan string, err error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, fmt.Errorf("starting callback server: %w", err)
	}
	port = listener.Addr().(*net.TCPAddr).Port

	ch := make(chan string, 1)
	srv := &http.Server{}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "missing token", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `<html><body><h2>Connected!</h2><p>You can close this tab and return to the terminal.</p></body></html>`)
		ch <- token
		go func() {
			time.Sleep(100 * time.Millisecond)
			srv.Shutdown(context.Background())
		}()
	})

	srv.Handler = mux

	go func() {
		srv.Serve(listener)
	}()

	// Auto-shutdown on timeout
	go func() {
		time.Sleep(timeout)
		srv.Shutdown(context.Background())
		select {
		case ch <- "":
		default:
		}
	}()

	return port, ch, nil
}
