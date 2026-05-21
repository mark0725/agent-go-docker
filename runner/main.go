package main

import (
	"context"
	"crypto/subtle"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/mark0725/agent-go-docker/runner/internal/api"
	"github.com/mark0725/agent-go-docker/runner/internal/config"
	"github.com/mark0725/agent-go-docker/runner/internal/docker"
)

//go:embed frontend/*
var frontendFS embed.FS

func main() {
	for _, p := range []string{".env", "runner.env"} {
		if err := config.LoadEnvFile(p); err != nil {
			log.Printf("warning: load %s: %v", p, err)
		}
	}

	cfg := config.Load()

	ports := docker.NewPortAllocator(cfg.PortRangeStart, cfg.PortRangeEnd)
	mgr, err := docker.NewManager(cfg, ports)
	if err != nil {
		log.Fatalf("docker manager: %v", err)
	}
	defer mgr.Close()

	if err := mgr.RecoverState(context.Background()); err != nil {
		log.Printf("warning: failed to recover state: %v", err)
	}

	handler := api.NewHandler(mgr)
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/agents", handler.CreateAgent)
	mux.HandleFunc("GET /api/agents", handler.ListAgents)
	mux.HandleFunc("GET /api/agents/{id}", handler.GetAgent)
	mux.HandleFunc("PUT /api/agents/{id}", handler.UpdateAgent)
	mux.HandleFunc("DELETE /api/agents/{id}", handler.RemoveAgent)
	mux.HandleFunc("POST /api/agents/{id}/restart", handler.RestartAgent)
	mux.HandleFunc("GET /api/config", handler.GetConfig)
	mux.HandleFunc("GET /api/lists/agents", handler.ListAgentIDs)
	mux.HandleFunc("GET /api/lists/projects", handler.ListProjects)
	mux.HandleFunc("GET /api/lists/workspaces", handler.ListWorkspaces)
	mux.Handle("/proxy/{id}/", handler.ProxyAgent())

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	frontendSub, _ := fs.Sub(frontendFS, "frontend")
	mux.Handle("/", http.FileServer(http.FS(frontendSub)))

	log.Printf("Agent Runner listening on %s", cfg.ListenAddr)
	log.Printf("Agent image: %s", cfg.AgentImage)
	log.Printf("Port range: %d-%d", cfg.PortRangeStart, cfg.PortRangeEnd)
	if cfg.AuthToken != "" {
		log.Printf("Auth: token required (Authorization / cookie / ?token=)")
	} else {
		log.Printf("Auth: disabled (open access)")
	}
	if err := http.ListenAndServe(cfg.ListenAddr, withAuth(cfg.AuthToken, mux)); err != nil {
		log.Fatal(err)
	}
}

const authCookieName = "runner_token"

// withAuth gates every route except /health behind a shared token. When the
// token is empty auth is disabled. Accepted credentials, in order:
//   1. Authorization: Bearer <token>
//   2. cookie runner_token=<token>
//   3. ?token=<token> query param (promoted to a cookie on first match so
//      embedded iframes/WebSocket upgrades stay authenticated)
func withAuth(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		if checkAuth(r, token) {
			if q := r.URL.Query().Get("token"); q != "" {
				http.SetCookie(w, &http.Cookie{
					Name:     authCookieName,
					Value:    q,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
				})
			}
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/proxy/") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`<!doctype html><meta charset=utf-8><title>Runner login</title>
<style>body{font-family:system-ui;background:#0f0f0f;color:#eee;display:flex;align-items:center;justify-content:center;height:100vh;margin:0}form{background:#1a1a2e;padding:24px 28px;border-radius:10px;border:1px solid #2a2a3e;min-width:280px}input{width:100%;padding:8px 10px;margin-top:6px;border-radius:6px;border:1px solid #3a3a5e;background:#0f0f1a;color:#eee}button{margin-top:14px;width:100%;padding:8px;border-radius:6px;border:0;background:#6c5ce7;color:#fff;cursor:pointer;font-weight:600}h1{font-size:16px;margin:0 0 12px}</style>
<form method=GET action=/><h1>Runner token</h1><label>Token<input name=token type=password autofocus></label><button>Sign in</button></form>`))
	})
}

func checkAuth(r *http.Request, token string) bool {
	if h := r.Header.Get("Authorization"); h != "" {
		if v, ok := strings.CutPrefix(h, "Bearer "); ok && tokenEqual(v, token) {
			return true
		}
	}
	if c, err := r.Cookie(authCookieName); err == nil && tokenEqual(c.Value, token) {
		return true
	}
	if q := r.URL.Query().Get("token"); q != "" && tokenEqual(q, token) {
		return true
	}
	return false
}

func tokenEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
