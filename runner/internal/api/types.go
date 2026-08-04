package api

import "time"

// EnvVar is a single KEY=VALUE pair the user wants injected into the agent
// container. Custom env vars are appended after the runner's own defaults
// (HOST_UID, AGENT_ID, …), so a name collision will override the runner's
// value inside the container.
type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type CreateAgentRequest struct {
	Name               string   `json:"name"`
	AgentID            string   `json:"agentId,omitempty"`
	AgentType          string   `json:"agentType,omitempty"`
	Variant            string   `json:"variant,omitempty"`
	Image              string   `json:"image,omitempty"`
	ProjectID          string   `json:"projectId,omitempty"`
	WorkspaceID        string   `json:"workspaceId,omitempty"`
	ProjectHome        string   `json:"projectHome,omitempty"`
	ClaudeHome         string   `json:"claudeHome,omitempty"`
	CodexHome          string   `json:"codexHome,omitempty"`
	AgentsHome         string   `json:"agentsHome,omitempty"`
	AgentsHub          string   `json:"agentsHub,omitempty"`
	ClaudeConfig       string   `json:"claudeConfig,omitempty"`
	AnthropicAuthToken string   `json:"anthropicAuthToken,omitempty"`
	AnthropicBaseURL   string   `json:"anthropicBaseUrl,omitempty"`
	ClaudeArgs         []string `json:"claudeArgs,omitempty"`
	AgentArgs          []string `json:"agentArgs,omitempty"`
	ExtraEnv           []EnvVar `json:"extraEnv,omitempty"`
}

type AgentResponse struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	AgentID         string   `json:"agentId,omitempty"`
	AgentType       string   `json:"agentType"`
	Status          string   `json:"status"`
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
	ClaudeConfig    string   `json:"claudeConfig,omitempty"`
	ClaudeArgs      []string `json:"claudeArgs,omitempty"`
	AgentArgs       []string `json:"agentArgs,omitempty"`
	ExtraEnv        []EnvVar `json:"extraEnv,omitempty"`
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
	HostHome          string `json:"hostHome"`
	AgentID           string `json:"agentId,omitempty"`
	ProjectRoot       string `json:"projectRoot"`
	ProjectHome       string `json:"projectHome,omitempty"`
	ClaudeHome        string `json:"claudeHome,omitempty"`
	CodexHome         string `json:"codexHome,omitempty"`
	AgentsHome        string `json:"agentsHome,omitempty"`
	AgentsHub         string `json:"agentsHub,omitempty"`
	ClaudeConfig      string `json:"claudeConfig,omitempty"`
	ImageRegistry     string `json:"imageRegistry"`
	ImageTag          string `json:"imageTag"`
	HasAuthDefault    bool   `json:"hasAuthDefault,omitempty"`
	HasBaseURLDefault bool   `json:"hasBaseUrlDefault,omitempty"`
	AnthropicBaseURL  string `json:"anthropicBaseUrl,omitempty"`
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
