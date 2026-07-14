// Command timetools runs the time service behind timetools.io: a world
// clock you can curl. See the README for the full endpoint list.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/admk-studio/timetools.io/internal/server"
)

// version is stamped by the linker; see the Makefile.
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	healthCheck := flag.Bool("health", false, "probe a running server and exit 0 if healthy")
	flag.Parse()

	if *showVersion {
		fmt.Println("timetools " + version)
		return
	}

	cfg := server.FromEnv()
	cfg.Version = version

	// The scratch image has no shell or curl, so the binary doubles as
	// its own health probe: HEALTHCHECK ["/timetools", "-health"].
	if *healthCheck {
		os.Exit(probe(cfg.Addr))
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	srv, err := server.New(cfg, log)
	if err != nil {
		log.Error("startup failed", "err", err)
		os.Exit(1)
	}

	httpSrv := srv.HTTPServer()
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpSrv.ListenAndServe()
	}()
	log.Info("timetools listening", "addr", cfg.Addr, "version", version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		log.Error("server stopped", "err", err)
		os.Exit(1)
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("shutdown failed", "err", err)
		os.Exit(1)
	}
}

func probe(addr string) int {
	// ":8080" needs a host to dial.
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + addr + "/health")
	if err != nil {
		fmt.Fprintln(os.Stderr, "unhealthy:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "unhealthy: status", resp.StatusCode)
		return 1
	}
	return 0
}
