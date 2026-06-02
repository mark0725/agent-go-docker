package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mark0725/agent-go-docker/runner/internal/config"
)

// NetworkName is the user-defined bridge network every agent container is
// attached to. The runner creates it on startup if it doesn't exist, then
// reverse-proxies agent ttyd traffic over it. The runner container itself
// must also be on this network (or otherwise able to resolve container IDs
// on it) — start it with `--network=agents-net` to satisfy this.
const NetworkName = "agents-net"

// AgentTtydPort is the (single, fixed) container port that every agent's
// ttyd binds to. The port is never published to the host — the runner
// reverse-proxies all ttyd traffic over NetworkName.
const AgentTtydPort = 7681

type Agent struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	AgentID         string    `json:"agentId,omitempty"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	Image           string    `json:"image"`
	Variant         string    `json:"variant,omitempty"`
	ProjectID       string    `json:"projectId,omitempty"`
	WorkspaceID     string    `json:"workspaceId,omitempty"`
	ProjectHome     string    `json:"projectHome,omitempty"`
	ClaudeHome      string    `json:"claudeHome,omitempty"`
	CodexHome       string    `json:"codexHome,omitempty"`
	AgentsHome      string    `json:"agentsHome,omitempty"`
	AgentsHub       string    `json:"agentsHub,omitempty"`
	ClaudeConfig    string    `json:"claudeConfig,omitempty"`
	ClaudeArgs      []string  `json:"claudeArgs,omitempty"`
	ExtraEnv        []EnvVar  `json:"extraEnv,omitempty"`
	HasAuthOverride bool      `json:"hasAuthOverride,omitempty"`
	HasBaseOverride bool      `json:"hasBaseOverride,omitempty"`
}

// EnvVar is a user-supplied KEY=VALUE pair injected into the container.
type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Manager struct {
	cfg *config.Config
}

func NewManager(ctx context.Context, cfg *config.Config) (*Manager, error) {
	m := &Manager{cfg: cfg}
	if err := m.EnsureNetwork(ctx, NetworkName); err != nil {
		return nil, fmt.Errorf("ensure network %s: %w", NetworkName, err)
	}
	return m, nil
}

func (m *Manager) Config() *config.Config { return m.cfg }

// EnsureNetwork creates the named user-defined bridge network if it does
// not already exist. The check is done via `docker network inspect` so the
// call is idempotent across runner restarts.
func (m *Manager) EnsureNetwork(ctx context.Context, name string) error {
	if err := m.dockerRun(ctx, "network", "inspect", name); err == nil {
		return nil
	}
	// `docker network create` is also idempotent in practice (it errors on
	// "already exists"), but the inspect above lets us avoid that error
	// log on every startup.
	if err := m.dockerRun(ctx, "network", "create", name); err != nil {
		// Race: another process created it between inspect and create.
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return err
	}
	return nil
}

type CreateAgentOpts struct {
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
	ClaudeConfig       string   `json:"claudeConfig,omitempty"`
	AnthropicAuthToken string   `json:"anthropicAuthToken,omitempty"`
	AnthropicBaseURL   string   `json:"anthropicBaseUrl,omitempty"`
	ClaudeArgs         []string `json:"claudeArgs,omitempty"`
	// ExtraEnv holds user-defined KEY=VALUE pairs to inject into the
	// container. nil means "inherit from previous agent" (used by edit),
	// an empty (non-nil) slice means "explicitly clear", and a populated
	// slice replaces any prior values.
	ExtraEnv []EnvVar `json:"extraEnv,omitempty"`
}

// resolvedPaths captures the host-side directories the runner will mount into a
// new agent container, after applying CLI overrides and falling back to the
// agent-go default layout.
type resolvedPaths struct {
	ProjectHome  string
	ClaudeHome   string
	CodexHome    string
	AgentsHome   string
	AgentsHub    string
	ClaudeConfig string
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// validateExtraEnv trims and normalizes user-supplied env vars, returning an
// error on the first invalid entry. Empty keys are rejected; values may be
// empty (which is meaningful — e.g. to clear an inherited var). Keys must
// match POSIX env-var naming (letters, digits, underscores; not starting
// with a digit).
func validateExtraEnv(in []EnvVar) ([]EnvVar, error) {
	out := make([]EnvVar, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for i, e := range in {
		key := strings.TrimSpace(e.Key)
		if key == "" {
			return nil, fmt.Errorf("extraEnv[%d]: key is empty", i)
		}
		for _, r := range key {
			ok := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
				(r >= '0' && r <= '9') || r == '_'
			if !ok {
				return nil, fmt.Errorf("extraEnv[%d]: invalid key %q (allowed: A-Z a-z 0-9 _)", i, key)
			}
		}
		if key[0] >= '0' && key[0] <= '9' {
			return nil, fmt.Errorf("extraEnv[%d]: key %q must not start with a digit", i, key)
		}
		if _, dup := seen[key]; dup {
			return nil, fmt.Errorf("extraEnv[%d]: duplicate key %q", i, key)
		}
		seen[key] = struct{}{}
		out = append(out, EnvVar{Key: key, Value: e.Value})
	}
	return out, nil
}

func (m *Manager) resolvePaths(opts CreateAgentOpts, agentID string) resolvedPaths {
	_ = agentID
	home := m.cfg.HostHome

	projectHome := firstNonEmpty(opts.ProjectHome, m.cfg.ProjectHome)
	if projectHome == "" {
		projectHome = m.cfg.ProjectRoot + "/" + opts.ProjectID
	}

	return resolvedPaths{
		ProjectHome:  projectHome,
		ClaudeHome:   firstNonEmpty(opts.ClaudeHome, m.cfg.ClaudeHome, home+"/.claude"),
		CodexHome:    firstNonEmpty(opts.CodexHome, m.cfg.CodexHome, home+"/.codex"),
		AgentsHome:   firstNonEmpty(opts.AgentsHome, m.cfg.AgentsHome, home+"/.agents"),
		AgentsHub:    firstNonEmpty(opts.AgentsHub, m.cfg.AgentsHub, home+"/.agents-hub"),
		ClaudeConfig: firstNonEmpty(opts.ClaudeConfig, m.cfg.ClaudeConfig),
	}
}

// resolveImage picks the docker image to use for an agent in priority order:
// explicit Image > Variant tag > runner default.
func (m *Manager) resolveImage(opts CreateAgentOpts) string {
	if opts.Image != "" {
		return opts.Image
	}
	if v := strings.TrimSpace(opts.Variant); v != "" && v != "default" {
		return fmt.Sprintf("%s:%s", m.cfg.ImageRegistry, v)
	}
	return m.cfg.AgentImage
}

func (m *Manager) CreateAgent(ctx context.Context, opts CreateAgentOpts) (*Agent, error) {
	if opts.Name == "" {
		opts.Name = fmt.Sprintf("agent-%d", time.Now().Unix())
	}

	image := m.resolveImage(opts)

	projectID := opts.ProjectID
	if projectID == "" {
		projectID = sanitizeName(opts.Name)
	}
	opts.ProjectID = projectID

	workspaceID := strings.TrimSpace(opts.WorkspaceID)
	projectMount := "/workspace/" + projectID
	containerWorkspace := projectMount
	if workspaceID != "" {
		containerWorkspace = projectMount + "/" + workspaceID
	}

	paths := m.resolvePaths(opts, opts.Name)

	agentID := strings.TrimSpace(opts.AgentID)
	if agentID == "" {
		agentID = m.cfg.AgentID
	}
	if agentID == "" {
		agentID = opts.Name
	}
	opts.AgentID = agentID

	containerName := fmt.Sprintf("runner-%s-%d", sanitizeName(opts.Name), time.Now().UnixMilli())

	// Compose the claude command with optional extra args (e.g. --resume <id>).
	claudeCmdTokens := []string{
		"claude",
		"--dangerously-skip-permissions",
		"--allowedTools", "Edit,Write,Bash",
	}
	claudeCmdTokens = append(claudeCmdTokens, opts.ClaudeArgs...)
	claudeCmd := joinShellQuoted(claudeCmdTokens)
	wrappedClaudeCmd := "bash -lc " + shellSingleQuote(". /tmp/agent-env.sh && exec "+claudeCmd)

	startupScript := buildStartupScript(wrappedClaudeCmd, containerWorkspace, AgentTtydPort)

	authToken := opts.AnthropicAuthToken
	hasAuthOverride := authToken != ""
	if !hasAuthOverride {
		authToken = m.cfg.AnthropicAuthToken
	}
	baseURL := opts.AnthropicBaseURL
	hasBaseOverride := baseURL != ""
	if !hasBaseOverride {
		baseURL = m.cfg.AnthropicBaseURL
	}

	claudeArgsLabel, _ := json.Marshal(opts.ClaudeArgs)
	extraEnv, err := validateExtraEnv(opts.ExtraEnv)
	if err != nil {
		return nil, err
	}
	extraEnvLabel, _ := json.Marshal(extraEnv)

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--user", "0",
		"--network", NetworkName,
		"--label", LabelManaged + "=true",
		"--label", LabelName + "=" + opts.Name,
		"--label", LabelAgentID + "=" + agentID,
		"--label", LabelVariant + "=" + opts.Variant,
		"--label", LabelProjectID + "=" + projectID,
		"--label", LabelWorkspaceID + "=" + workspaceID,
		"--label", LabelImage + "=" + image,
		"--label", LabelClaudeArgs + "=" + string(claudeArgsLabel),
		"--label", LabelHasAuthOvr + "=" + boolStr(hasAuthOverride),
		"--label", LabelHasBaseOvr + "=" + boolStr(hasBaseOverride),
		"--label", LabelProjectHome + "=" + paths.ProjectHome,
		"--label", LabelClaudeHome + "=" + paths.ClaudeHome,
		"--label", LabelCodexHome + "=" + paths.CodexHome,
		"--label", LabelAgentsHome + "=" + paths.AgentsHome,
		"--label", LabelAgentsHub + "=" + paths.AgentsHub,
		"--label", LabelExtraEnv + "=" + string(extraEnvLabel),
		"--label", LabelClaudeConfig + "=" + paths.ClaudeConfig,
		"-e", fmt.Sprintf("HOST_UID=%d", m.cfg.HostUID),
		"-e", fmt.Sprintf("HOST_GID=%d", m.cfg.HostGID),
		"-e", fmt.Sprintf("TZ=%s", m.cfg.TZ),
		"-e", "HOME=/home/node",
		"-e", "XDG_RUNTIME_DIR=/tmp/runtime-node",
		"-e", "AGENT_ID=" + agentID,
		"-e", "PROJECT_ID=" + projectID,
		"-e", "WORKSPACE_ID=" + workspaceID,
		"-e", "AGENT_WORKSPACE=" + containerWorkspace,
		"-e", "PROJECT_HOME=" + projectMount,
		"-e", "CLAUDE_HOME=/home/node/.claude",
		"-e", "CODEX_HOME=/home/node/.codex",
		"-e", "AGENTS_HOME=/home/node/.agents",
		"-e", "AGENTS_HUB=/home/node/.agents-hub",
	}

	for _, e := range []struct{ k, v string }{
		{"ANTHROPIC_AUTH_TOKEN", authToken},
		{"ANTHROPIC_BASE_URL", baseURL},
		{"PROXY_URL", m.cfg.ProxyURL},
		{"HTTP_PROXY", m.cfg.HTTPProxy},
		{"HTTPS_PROXY", m.cfg.HTTPSProxy},
	} {
		if e.v != "" {
			args = append(args, "-e", fmt.Sprintf("%s=%s", e.k, e.v))
		}
	}

	// User-supplied env vars go last so they can intentionally override the
	// runner defaults above (e.g. ANTHROPIC_BASE_URL).
	for _, e := range extraEnv {
		args = append(args, "-e", fmt.Sprintf("%s=%s", e.Key, e.Value))
	}

	args = append(args,
		"-v", "node_home_"+sanitizeName(opts.Name)+":/home/node",
		"-v", "/etc/localtime:/etc/localtime:ro",
		"-v", m.cfg.DockerSocket+":/var/run/docker.sock",
		"-v", paths.ProjectHome+":"+projectMount,
		"-v", paths.ClaudeHome+":/home/node/.claude",
		"-v", paths.CodexHome+":/home/node/.codex",
		"-v", paths.AgentsHome+":/home/node/.agents",
		"-v", paths.AgentsHub+":/home/node/.agents-hub",
	)
	// ClaudeConfig is optional: a host-side .claude.json file can be
	// bind-mounted into the agent to override the empty file that lives on
	// the node_home_<name> volume. Must be added before `image` in the
	// args — anything after `image` is a positional arg to the container
	// command, not a docker flag.
	if paths.ClaudeConfig != "" {
		args = append(args, "-v", paths.ClaudeConfig+":/home/node/.claude.json")
	}
	args = append(args,
		"-w", containerWorkspace,
		image,
		"bash", "-lc", startupScript,
	)

	containerID, err := m.dockerOutput(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}
	containerID = strings.TrimSpace(containerID)

	return &Agent{
		ID:              containerID[:12],
		Name:            opts.Name,
		AgentID:         agentID,
		Status:          "running",
		CreatedAt:       time.Now(),
		Image:           image,
		Variant:         opts.Variant,
		ProjectID:       projectID,
		WorkspaceID:     workspaceID,
		ProjectHome:     paths.ProjectHome,
		ClaudeHome:      paths.ClaudeHome,
		CodexHome:       paths.CodexHome,
		AgentsHome:      paths.AgentsHome,
		AgentsHub:       paths.AgentsHub,
		ClaudeConfig:    paths.ClaudeConfig,
		ClaudeArgs:      opts.ClaudeArgs,
		ExtraEnv:        extraEnv,
		HasAuthOverride: hasAuthOverride,
		HasBaseOverride: hasBaseOverride,
	}, nil
}

// RecreateAgent removes the existing container and creates a new one. Fields
// left empty in opts are inherited from the previous container's labels so
// callers can submit partial updates.
func (m *Manager) RecreateAgent(ctx context.Context, id string, opts CreateAgentOpts) (*Agent, error) {
	prev, err := m.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}

	if opts.Name == "" {
		opts.Name = prev.Name
	}
	if opts.AgentID == "" {
		opts.AgentID = prev.AgentID
	}
	if opts.Variant == "" {
		opts.Variant = prev.Variant
	}
	if opts.ProjectID == "" {
		opts.ProjectID = prev.ProjectID
	}
	if opts.WorkspaceID == "" {
		opts.WorkspaceID = prev.WorkspaceID
	}
	if opts.ProjectHome == "" {
		opts.ProjectHome = prev.ProjectHome
	}
	if opts.ClaudeHome == "" {
		opts.ClaudeHome = prev.ClaudeHome
	}
	if opts.CodexHome == "" {
		opts.CodexHome = prev.CodexHome
	}
	if opts.AgentsHome == "" {
		opts.AgentsHome = prev.AgentsHome
	}
	if opts.AgentsHub == "" {
		opts.AgentsHub = prev.AgentsHub
	}
	if opts.ClaudeConfig == "" {
		opts.ClaudeConfig = prev.ClaudeConfig
	}
	if opts.ClaudeArgs == nil {
		opts.ClaudeArgs = prev.ClaudeArgs
	}
	if opts.ExtraEnv == nil {
		opts.ExtraEnv = prev.ExtraEnv
	}

	if err := m.RemoveAgent(ctx, prev.ID); err != nil {
		return nil, fmt.Errorf("remove previous container: %w", err)
	}

	return m.CreateAgent(ctx, opts)
}

func (m *Manager) RestartAgent(ctx context.Context, id string) error {
	agent, err := m.GetAgent(ctx, id)
	if err != nil {
		return err
	}
	if err := m.dockerRun(ctx, "restart", agent.ID); err != nil {
		return fmt.Errorf("restart container: %w", err)
	}
	return nil
}

func (m *Manager) ListAgents(ctx context.Context) ([]*Agent, error) {
	// First pass: just collect container IDs matching our managed label.
	// We intentionally avoid `--format` templates that interpolate labels
	// into a string — label values can contain quotes and commas (e.g.
	// JSON-encoded claude-args / extra-env), which corrupts any
	// string-based parse. `docker inspect` returns labels as a proper
	// JSON map, so we read that instead.
	idsOut, err := m.dockerOutput(ctx,
		"ps", "-a",
		"--filter", "label="+LabelManaged+"=true",
		"--format", "{{.ID}}",
	)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	idsOut = strings.TrimSpace(idsOut)
	if idsOut == "" {
		return []*Agent{}, nil
	}

	ids := strings.Split(idsOut, "\n")
	args := append([]string{"inspect", "--format", "{{json .}}"}, ids...)
	inspectOut, err := m.dockerOutput(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("inspect containers: %w", err)
	}

	var agents []*Agent
	for _, line := range strings.Split(inspectOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var raw struct {
			ID      string `json:"Id"`
			Created string `json:"Created"`
			State   struct {
				Status string `json:"Status"`
			} `json:"State"`
			Config struct {
				Image  string            `json:"Image"`
				Labels map[string]string `json:"Labels"`
			} `json:"Config"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		labels := raw.Config.Labels
		if labels == nil {
			labels = map[string]string{}
		}

		var claudeArgs []string
		if v := labels[LabelClaudeArgs]; v != "" {
			_ = json.Unmarshal([]byte(v), &claudeArgs)
		}

		var extraEnv []EnvVar
		if v := labels[LabelExtraEnv]; v != "" {
			_ = json.Unmarshal([]byte(v), &extraEnv)
		}

		createdAt, _ := time.Parse(time.RFC3339Nano, raw.Created)

		agents = append(agents, &Agent{
			ID:              shortID(raw.ID),
			Name:            labels[LabelName],
			AgentID:         labels[LabelAgentID],
			Status:          raw.State.Status,
			CreatedAt:       createdAt,
			Image:           raw.Config.Image,
			Variant:         labels[LabelVariant],
			ProjectID:       labels[LabelProjectID],
			WorkspaceID:     labels[LabelWorkspaceID],
			ProjectHome:     labels[LabelProjectHome],
			ClaudeHome:      labels[LabelClaudeHome],
			CodexHome:       labels[LabelCodexHome],
			AgentsHome:      labels[LabelAgentsHome],
			AgentsHub:       labels[LabelAgentsHub],
			ClaudeConfig:    labels[LabelClaudeConfig],
			ClaudeArgs:      claudeArgs,
			ExtraEnv:        extraEnv,
			HasAuthOverride: labels[LabelHasAuthOvr] == "true",
			HasBaseOverride: labels[LabelHasBaseOvr] == "true",
		})
	}
	return agents, nil
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func (m *Manager) GetAgent(ctx context.Context, id string) (*Agent, error) {
	if id == "" {
		return nil, fmt.Errorf("empty agent id")
	}
	agents, err := m.ListAgents(ctx)
	if err != nil {
		return nil, err
	}
	var match *Agent
	for _, a := range agents {
		if a.ID == id {
			return a, nil
		}
		if strings.HasPrefix(a.ID, id) {
			if match != nil {
				return nil, fmt.Errorf("agent id %q is ambiguous", id)
			}
			match = a
		}
	}
	if match != nil {
		return match, nil
	}
	return nil, fmt.Errorf("agent %s not found", id)
}

func (m *Manager) RemoveAgent(ctx context.Context, id string) error {
	agent, err := m.GetAgent(ctx, id)
	if err != nil {
		return err
	}

	_ = m.dockerRun(ctx, "stop", "-t", "10", agent.ID)

	if err := m.dockerRun(ctx, "rm", "-f", agent.ID); err != nil {
		if !strings.Contains(err.Error(), "No such container") {
			return fmt.Errorf("remove container: %w", err)
		}
	}

	if agent.Name != "" {
		volume := "node_home_" + sanitizeName(agent.Name)
		_ = m.dockerRun(ctx, "volume", "rm", "-f", volume)
	}

	return nil
}

func (m *Manager) Close() error { return nil }

func (m *Manager) dockerRun(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
}

func (m *Manager) dockerOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func sanitizeName(name string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, name))
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func joinShellQuoted(tokens []string) string {
	parts := make([]string, 0, len(tokens))
	for _, t := range tokens {
		parts = append(parts, shellSingleQuote(t))
	}
	return strings.Join(parts, " ")
}

// buildStartupScript renders the in-container bootstrap that starts a shpool
// session running claude, then exposes it via ttyd on the chosen port.
func buildStartupScript(wrappedClaudeCmd, workspace string, port int) string {
	return fmt.Sprintf(
		`export CLAUDE_CMD=%s; export AGENT_WORKSPACE=%s; export TERM="${TERM:-xterm-256color}"; export COLORTERM="${COLORTERM:-truecolor}"; export FORCE_COLOR="${FORCE_COLOR:-3}"; mkdir -p "$XDG_RUNTIME_DIR" && chmod 700 "$XDG_RUNTIME_DIR" && (umask 077; export -p > /tmp/agent-env.sh) && shpool attach -b -f --dir "$AGENT_WORKSPACE" -c "$CLAUDE_CMD" claude && for i in $(seq 1 50); do shpool list >/dev/null 2>&1 && break; sleep 0.1; done && printf '%%s\n' '#!/bin/bash' 'exec shpool attach -f claude' > /tmp/agent-cc-web-shpool && chmod +x /tmp/agent-cc-web-shpool && exec /usr/local/bin/ttyd -p %d -W /tmp/agent-cc-web-shpool`,
		shellSingleQuote(wrappedClaudeCmd),
		shellSingleQuote(workspace),
		port,
	)
}
