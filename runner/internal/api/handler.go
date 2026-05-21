package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/mark0725/agent-go-docker/runner/internal/docker"
)

type Handler struct {
	manager *docker.Manager
}

func NewHandler(mgr *docker.Manager) *Handler {
	return &Handler{manager: mgr}
}

func (h *Handler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	var req CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	agent, err := h.manager.CreateAgent(r.Context(), reqToOpts(req))
	if err != nil {
		log.Printf("create agent error: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, agentToResponse(agent))
}

func (h *Handler) UpdateAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing agent id")
		return
	}
	var req CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	agent, err := h.manager.RecreateAgent(r.Context(), id, reqToOpts(req))
	if err != nil {
		log.Printf("update agent error: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, agentToResponse(agent))
}

func (h *Handler) RestartAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing agent id")
		return
	}
	if err := h.manager.RestartAgent(r.Context(), id); err != nil {
		log.Printf("restart agent error: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarting"})
}

func reqToOpts(req CreateAgentRequest) docker.CreateAgentOpts {
	args := req.ClaudeArgs
	cleaned := args[:0]
	for _, a := range args {
		a = strings.TrimSpace(a)
		if a != "" {
			cleaned = append(cleaned, a)
		}
	}
	return docker.CreateAgentOpts{
		Name:               strings.TrimSpace(req.Name),
		AgentID:            strings.TrimSpace(req.AgentID),
		Variant:            strings.TrimSpace(req.Variant),
		Image:              strings.TrimSpace(req.Image),
		ProjectID:          strings.TrimSpace(req.ProjectID),
		WorkspaceID:        strings.TrimSpace(req.WorkspaceID),
		ProjectHome:        strings.TrimSpace(req.ProjectHome),
		ClaudeHome:         strings.TrimSpace(req.ClaudeHome),
		CodexHome:          strings.TrimSpace(req.CodexHome),
		AgentsHome:         strings.TrimSpace(req.AgentsHome),
		AgentsHub:          strings.TrimSpace(req.AgentsHub),
		AnthropicAuthToken: req.AnthropicAuthToken,
		AnthropicBaseURL:   req.AnthropicBaseURL,
		ClaudeArgs:         cleaned,
	}
}

func (h *Handler) ListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := h.manager.ListAgents(r.Context())
	if err != nil {
		log.Printf("list agents error: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resps := make([]AgentResponse, 0, len(agents))
	for _, a := range agents {
		resps = append(resps, agentToResponse(a))
	}
	writeJSON(w, http.StatusOK, ListAgentsResponse{Agents: resps})
}

func (h *Handler) GetAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing agent id")
		return
	}

	agent, err := h.manager.GetAgent(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, agentToResponse(agent))
}

func (h *Handler) RemoveAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing agent id")
		return
	}

	if err := h.manager.RemoveAgent(r.Context(), id); err != nil {
		log.Printf("remove agent error: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := h.manager.Config()
	resp := ConfigResponse{
		HostHome:          cfg.HostHome,
		AgentID:           cfg.AgentID,
		ProjectRoot:       cfg.ProjectRoot,
		ProjectHome:       cfg.ProjectHome,
		ClaudeHome:        cfg.ClaudeHome,
		CodexHome:         cfg.CodexHome,
		AgentsHome:        cfg.AgentsHome,
		AgentsHub:         cfg.AgentsHub,
		ImageRegistry:     cfg.ImageRegistry,
		ImageTag:          cfg.ImageTag,
		HasAuthDefault:    cfg.AnthropicAuthToken != "",
		HasBaseURLDefault: cfg.AnthropicBaseURL != "",
		AnthropicBaseURL:  cfg.AnthropicBaseURL,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListAgentIDs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, ListResponse{Items: listDirs("/data/hub/agents")})
}

func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	cfg := h.manager.Config()
	writeJSON(w, http.StatusOK, ListResponse{Items: listDirs(cfg.ProjectRoot)})
}

func (h *Handler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	cfg := h.manager.Config()
	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		writeJSON(w, http.StatusOK, ListResponse{})
		return
	}
	dir := cfg.ProjectRoot + "/" + projectID
	writeJSON(w, http.StatusOK, ListResponse{Items: listDirs(dir)})
}

// listDirs returns the names of immediate subdirectories under dir.
// Returns an empty slice on error.
func listDirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return names
}

func (h *Handler) ProxyAgent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "missing agent id")
			return
		}

		agent, err := h.manager.GetAgent(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		target, _ := url.Parse(fmt.Sprintf("http://%s:%d", h.manager.Config().ProxyHost, agent.TTYDPort))

		proxy := httputil.NewSingleHostReverseProxy(target)
		origDirector := proxy.Director
		prefix := "/proxy/" + id
		proxy.Director = func(req *http.Request) {
			origDirector(req)
			// Strip /proxy/{id} prefix from path so ttyd sees its own root.
			req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
			// Let net/url re-derive RawPath from Path; otherwise Path and
			// RawPath disagree, breaking downstream URL parsing.
			req.URL.RawPath = ""
			req.Host = target.Host
		}

		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy error for agent %s: %v", id, err)
			writeError(w, http.StatusBadGateway, "agent terminal unavailable")
		}

		proxy.ServeHTTP(w, r)
	}
}

func agentToResponse(a *docker.Agent) AgentResponse {
	return AgentResponse{
		ID:              a.ID,
		Name:            a.Name,
		AgentID:         a.AgentID,
		Status:          a.Status,
		TTYDPort:        a.TTYDPort,
		TTYDURL:         "/proxy/" + a.ID + "/",
		CreatedAt:       formatTime(a.CreatedAt),
		Image:           a.Image,
		Variant:         a.Variant,
		ProjectID:       a.ProjectID,
		WorkspaceID:     a.WorkspaceID,
		ProjectHome:     a.ProjectHome,
		ClaudeHome:      a.ClaudeHome,
		CodexHome:       a.CodexHome,
		AgentsHome:      a.AgentsHome,
		AgentsHub:       a.AgentsHub,
		ClaudeArgs:      a.ClaudeArgs,
		HasAuthOverride: a.HasAuthOverride,
		HasBaseOverride: a.HasBaseOverride,
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}
