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

	// 2. Check CI environment variables
	if b := detectBranchFromCI(); b != "" {
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
		// HEAD means detached HEAD — try to resolve via git branch --contains
		if branch == "HEAD" {
			if resolved := resolveBranchFromDetachedHead(); resolved != "" {
				return resolved
			}
		}
	}

	// 4. Default to main
	return "main"
}

func detectBranchFromCI() string {
	ciEnvs := []string{
		"GITHUB_HEAD_REF",
		"GITHUB_REF_NAME",
		"CI_COMMIT_BRANCH",
		"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME",
		"BITBUCKET_BRANCH",
		"TRAVIS_BRANCH",
		"CIRCLE_BRANCH",
	}

	for _, env := range ciEnvs {
		if b := os.Getenv(env); b != "" {
			return b
		}
	}

	// Special handling for some CIs
	if b := os.Getenv("GIT_BRANCH"); b != "" {
		return strings.TrimPrefix(b, "origin/")
	}
	if b := os.Getenv("BRANCH_NAME"); b != "" {
		return b
	}
	if b := os.Getenv("BUILD_SOURCEBRANCH"); b != "" {
		return strings.TrimPrefix(b, "refs/heads/")
	}

	return ""
}

func resolveBranchFromDetachedHead() string {
	cmd := exec.Command("git", "branch", "-a", "--contains", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// Skip detached HEAD marker and empty lines
		if line == "" || strings.HasPrefix(line, "(") || strings.HasPrefix(line, "* (") {
			continue
		}
		line = strings.TrimPrefix(line, "* ")
		// Prefer local branches; strip remotes/origin/ prefix
		if strings.HasPrefix(line, "remotes/origin/") {
			candidate := strings.TrimPrefix(line, "remotes/origin/")
			if candidate != "HEAD" {
				return candidate
			}
			continue
		}
		// Local branch
		return line
	}
	return ""
}
