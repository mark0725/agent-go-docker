package api

import "time"

type CreateAgentRequest struct {
	Name               string   `json:"name"`
	AgentID            string   `json:"agentId,omitempty"`
	Variant            string   `json:"variant,omitempty"`
	Image              string   `json:"image,omitempty"`
	ProjectID          string   `json:"projectId,omitempty"`
	WorkspaceID        string   `json:"workspaceId,omitempty"`
	ProjectHome        string   `json:"projectHome,omitempty"`
	ClaudeHome         string   `json:"claudeHome,omitempty"`
	CodexHome          string   `json:"codexHome,omitempty"`
	AgentsHome         string   `json:"agentsHome,omitempty"`
	AgentsHub          string   `json:"agentsHub,omitempty"`
	AnthropicAuthToken string   `json:"anthropicAuthToken,omitempty"`
	AnthropicBaseURL   string   `json:"anthropicBaseUrl,omitempty"`
	ClaudeArgs         []string `json:"claudeArgs,omitempty"`
}

type AgentResponse struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	AgentID         string   `json:"agentId,omitempty"`
	Status          string   `json:"status"`
	TTYDPort        int      `json:"ttydPort"`
	TTYDURL         string   `json:"ttydUrl"`
	CreatedAt       string   `json:"createdAt"`
	Image           string   `json:"image"`
	Variant         string   `json:"variant,omitempty"`
	ProjectID       string   `json:"projectId,omitempty"`
	WorkspaceID     string   `json:"workspaceId,omitempty"`
	ProjectHome     string   `json:"projectHome,omitempty"`
	ClaudeHome      string   `json:"claudeHome,omitempty"`
	CodexHome       string   `json:"codexHome,omitempty"`
	AgentsHome      string   `json:"agentsHome,omitempty"`
	AgentsHub       string   `json:"agentsHub,omitempty"`
	ClaudeArgs      []string `json:"claudeArgs,omitempty"`
	HasAuthOverride bool     `json:"hasAuthOverride,omitempty"`
	HasBaseOverride bool     `json:"hasBaseOverride,omitempty"`
}

type ListAgentsResponse struct {
	Agents []AgentResponse `json:"agents"`
}

type ListResponse struct {
	Items []string `json:"items"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// ConfigResponse exposes runtime defaults used by the frontend to render
// placeholders that reflect what a path will actually be if left blank.
type ConfigResponse struct {
	HostHome           string `json:"hostHome"`
	AgentID            string `json:"agentId,omitempty"`
	ProjectRoot        string `json:"projectRoot"`
	ProjectHome        string `json:"projectHome,omitempty"`
	ClaudeHome         string `json:"claudeHome,omitempty"`
	CodexHome          string `json:"codexHome,omitempty"`
	AgentsHome         string `json:"agentsHome,omitempty"`
	AgentsHub          string `json:"agentsHub,omitempty"`
	ImageRegistry      string `json:"imageRegistry"`
	ImageTag           string `json:"imageTag"`
	HasAuthDefault     bool   `json:"hasAuthDefault,omitempty"`
	HasBaseURLDefault  bool   `json:"hasBaseUrlDefault,omitempty"`
	AnthropicBaseURL   string `json:"anthropicBaseUrl,omitempty"`
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
