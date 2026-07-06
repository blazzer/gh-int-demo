package main

import (
	"context"
	"encoding/json"
	"expvar"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/blazzer/gh-int-demo/internal/auth"
	"github.com/blazzer/gh-int-demo/internal/github"
	"github.com/blazzer/gh-int-demo/internal/mcptools"
	"github.com/blazzer/gh-int-demo/internal/obs"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	version   = "dev"
	commit    = "none"
	transport = flag.String("transport", "http", "MCP transport: http or stdio")
	addr      = flag.String("addr", ":8080", "HTTP listen address")
)

func main() {
	flag.Parse()
	if port := os.Getenv("PORT"); port != "" && *addr == ":8080" {
		*addr = fmt.Sprintf(":%s", port)
	}

	logger := obs.NewLogger()
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mcpServer := newMCPServer(logger)
	switch *transport {
	case "stdio":
		if err := mcpServer.Run(ctx, &mcp.StdioTransport{}); err != nil {
			logger.Error("stdio server failed", "error", err)
			os.Exit(1)
		}
	case "http":
		if err := runHTTPServer(ctx, logger, mcpServer); err != nil {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	default:
		logger.Error("unknown transport", "transport", *transport)
		os.Exit(2)
	}
}

func newMCPServer(logger *slog.Logger) *mcp.Server {
	var ghClient github.Lister
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		ghClient = github.NewClient(token)
	}
	return newMCPServerWithClients(logger, ghClient, github.DefaultClientFactory())
}

func newMCPServerWithClients(logger *slog.Logger, defaultClient github.Lister, factory github.ClientFactory) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "gh-int-demo",
		Version: version,
	}, nil)

	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			obs.IncMCPRequest(method)
			reqLogger := obs.LoggerFromContext(ctx)
			if reqLogger == nil {
				reqLogger = logger
			}
			reqLogger.Info("mcp request", "method", method, "request_id", obs.RequestIDFromContext(ctx))
			return next(ctx, method, req)
		}
	})

	mcptools.RegisterListRepositories(server, defaultClient, factory)
	mcptools.RegisterGetRepository(server, defaultClient, factory)
	return server
}

func runHTTPServer(ctx context.Context, logger *slog.Logger, mcpServer *mcp.Server) error {
	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return mcpServer
	}, nil)

	mcpStack := obs.RequestIDMiddleware(logger, obs.TokenMiddleware(auth.Middleware(mcpHandler)))

	mux := http.NewServeMux()
	mux.Handle("/healthz", obs.RequestIDMiddleware(logger, healthHandler(logger)))
	mux.Handle("/mcp", mcpStack)
	if os.Getenv("ENABLE_DEBUG_VARS") == "1" {
		mux.Handle("/debug/vars", expvar.Handler())
	}

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	logger.Info("server listening", "addr", *addr, "transport", "http", "auth_mode", auth.Mode())
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func healthHandler(logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info("health check", "request_id", obs.RequestIDFromContext(r.Context()))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"version": version,
			"commit":  commit,
		})
	})
}
