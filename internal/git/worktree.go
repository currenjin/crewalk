package git

import (
	"fmt"
	"os/exec"
)

type Manager struct {
	projectPath  string
	worktreeBase string
}

func NewManager(projectPath, worktreeBase string) *Manager {
	return &Manager{
		projectPath:  projectPath,
		worktreeBase: worktreeBase,
	}
}

func (m *Manager) CreateWorktree(ticketID, worktreePath string) error {
	branchName := fmt.Sprintf("feature/%s", ticketID)

	cmd := exec.Command("git", "-C", m.projectPath,
		"worktree", "add", "-b", branchName,
		worktreePath, "origin/develop",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add: %w\n%s", err, out)
	}
	return nil
}

func (m *Manager) RemoveWorktree(worktreePath string) error {
	cmd := exec.Command("git", "-C", m.projectPath,
		"worktree", "remove", "--force", worktreePath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %w\n%s", err, out)
	}
	return nil
}
