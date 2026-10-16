package onboard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyEmbeddedToTargetUsesStructuredAgentFiles(t *testing.T) {
	targetDir := t.TempDir()

	if err := copyEmbeddedToTarget(targetDir); err != nil {
		t.Fatalf("copyEmbeddedToTarget() error = %v", err)
	}

	agentPath := filepath.Join(targetDir, "AGENT.md")
	if _, err := os.Stat(agentPath); err != nil {
		t.Fatalf("expected %s to exist: %v", agentPath, err)
	}

	// Re-pinned to the embedded workspace/ tree as shipped today: AGENT.md plus
	// skills/adoption/* — SOUL.md and USER.md no longer ship (removed by aa5065e).
	wantFiles := []string{
		filepath.Join(targetDir, "skills", "adoption", "SKILL.md"),
		filepath.Join(targetDir, "skills", "adoption", "accept.mjs"),
		filepath.Join(targetDir, "skills", "adoption", "seek.mjs"),
	}
	for _, path := range wantFiles {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	for _, removedName := range []string{"SOUL.md", "USER.md", "AGENTS.md", "IDENTITY.md"} {
		removedPath := filepath.Join(targetDir, removedName)
		if _, err := os.Stat(removedPath); !os.IsNotExist(err) {
			t.Fatalf("expected removed file %s to be absent, got err=%v", removedPath, err)
		}
	}
}
