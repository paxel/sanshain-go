package git

import (
	"os"
	"os/exec"
	"strings"
)

func GetCurrentBranch() string {
	// 1. Check SANSHAIN_BRANCH env var
	if b := os.Getenv("SANSHAIN_BRANCH"); b != "" {
		return b
	}

	// 2. Check common CI env vars
	if b := os.Getenv("GITHUB_REF_NAME"); b != "" {
		return b
	}
	if b := os.Getenv("CI_COMMIT_REF_NAME"); b != "" {
		return b
	}
	if b := os.Getenv("GIT_BRANCH"); b != "" {
		return b
	}

	// 3. Try git command
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err == nil {
		branch := strings.TrimSpace(string(out))
		if branch != "" && branch != "HEAD" {
			return branch
		}
	}

	// 4. Default to main
	return "main"
}
