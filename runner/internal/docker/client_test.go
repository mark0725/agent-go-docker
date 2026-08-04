package docker

import (
	"strings"
	"testing"
)

func TestBuildStartupScriptUsesSelectedSession(t *testing.T) {
	script := buildStartupScript("bash -lc 'exec codex --dangerously-bypass-approvals-and-sandbox'", "/workspace/demo", 7681, "codex")

	for _, want := range []string{
		"export AGENT_CMD=",
		"export AGENT_SESSION='codex'",
		`shpool attach -b -f --dir "$AGENT_WORKSPACE" -c "$AGENT_CMD" "$AGENT_SESSION"`,
		"exec shpool attach -f $AGENT_SESSION",
		"ttyd -p 7681",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("startup script missing %q:\n%s", want, script)
		}
	}
}

func TestAgentCommandDefaults(t *testing.T) {
	tests := []struct {
		agentType string
		want      []string
	}{
		{"claude", []string{"claude", "--dangerously-skip-permissions", "--allowedTools", "Edit,Write,Bash"}},
		{"codex", []string{"codex", "--dangerously-bypass-approvals-and-sandbox"}},
	}

	for _, tt := range tests {
		t.Run(tt.agentType, func(t *testing.T) {
			got := defaultAgentCommand(tt.agentType)
			if strings.Join(got, "\x00") != strings.Join(tt.want, "\x00") {
				t.Fatalf("defaultAgentCommand(%q) = %#v, want %#v", tt.agentType, got, tt.want)
			}
		})
	}
}

func TestNormalizeAgentType(t *testing.T) {
	for input, want := range map[string]string{"": "claude", " CLAUDE ": "claude", "Codex": "codex"} {
		got, err := normalizeAgentType(input)
		if err != nil || got != want {
			t.Fatalf("normalizeAgentType(%q) = %q, %v; want %q, nil", input, got, err, want)
		}
	}
	if _, err := normalizeAgentType("other"); err == nil {
		t.Fatal("normalizeAgentType(other) unexpectedly succeeded")
	}
}
