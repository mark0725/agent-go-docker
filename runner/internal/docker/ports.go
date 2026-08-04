package docker

const (
	LabelManaged     = "agent-go-runner.managed"
	LabelName        = "agent-go-runner.name"
	LabelAgentID     = "agent-go-runner.agent-id"
	LabelAgentType   = "agent-go-runner.agent-type"
	LabelVariant     = "agent-go-runner.variant"
	LabelProjectID   = "agent-go-runner.project-id"
	LabelWorkspaceID = "agent-go-runner.workspace-id"
	LabelImage       = "agent-go-runner.image"
	LabelClaudeArgs  = "agent-go-runner.claude-args"
	LabelAgentArgs   = "agent-go-runner.agent-args"
	LabelHasAuthOvr  = "agent-go-runner.auth-override"
	LabelHasBaseOvr  = "agent-go-runner.base-override"
	LabelProjectHome = "agent-go-runner.project-home"
	LabelClaudeHome  = "agent-go-runner.claude-home"
	LabelCodexHome   = "agent-go-runner.codex-home"
	LabelAgentsHome  = "agent-go-runner.agents-home"
	LabelAgentsHub   = "agent-go-runner.agents-hub"
	// LabelExtraEnv stores the user-supplied KEY=VALUE pairs as a JSON
	// array of {key, value} objects so values may contain '=' freely.
	LabelExtraEnv = "agent-go-runner.extra-env"
	// LabelClaudeConfig stores the host-side .claude.json path that was
	// bind-mounted into the agent container at /home/node/.claude.json.
	// Empty means "no host override" (the container's own copy on the
	// node_home_<name> named volume is used).
	LabelClaudeConfig = "agent-go-runner.claude-config"
)
