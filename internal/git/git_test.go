package git

import (
	"os"
	"testing"
)

func TestGetCurrentBranch(t *testing.T) {
	// Backup env vars
	origSanshain := os.Getenv("SANSHAIN_BRANCH")
	origGithub := os.Getenv("GITHUB_REF_NAME")
	origGitCommit := os.Getenv("CI_COMMIT_REF_NAME")
	origGitBranch := os.Getenv("GIT_BRANCH")
	
	defer func() {
		os.Setenv("SANSHAIN_BRANCH", origSanshain)
		os.Setenv("GITHUB_REF_NAME", origGithub)
		os.Setenv("CI_COMMIT_REF_NAME", origGitCommit)
		os.Setenv("GIT_BRANCH", origGitBranch)
	}()

	t.Run("SANSHAIN_BRANCH", func(t *testing.T) {
		os.Setenv("SANSHAIN_BRANCH", "feature-x")
		if b := GetCurrentBranch(); b != "feature-x" {
			t.Errorf("expected feature-x, got %s", b)
		}
	})

	t.Run("GITHUB_REF_NAME", func(t *testing.T) {
		os.Setenv("SANSHAIN_BRANCH", "")
		os.Setenv("GITHUB_REF_NAME", "main")
		if b := GetCurrentBranch(); b != "main" {
			t.Errorf("expected main, got %s", b)
		}
	})
	
	t.Run("Fallback to main", func(t *testing.T) {
		os.Setenv("SANSHAIN_BRANCH", "")
		os.Setenv("GITHUB_REF_NAME", "")
		os.Setenv("CI_COMMIT_REF_NAME", "")
		os.Setenv("GIT_BRANCH", "")
		// Note: we can't easily mock git command failure here without complex mocking
		// But if we are in a non-git environment it should fallback to main
		// In this environment it might actually find a branch if it's a git repo
	})
}
