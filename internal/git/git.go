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
	// GitHub Actions
	if b := os.Getenv("GITHUB_HEAD_REF"); b != "" {
		return b
	}
	if b := os.Getenv("GITHUB_REF_NAME"); b != "" {
		return b
	}

	// GitLab CI
	if b := os.Getenv("CI_COMMIT_BRANCH"); b != "" {
		return b
	}
	if b := os.Getenv("CI_MERGE_REQUEST_SOURCE_BRANCH_NAME"); b != "" {
		return b
	}

	// Jenkins
	if b := os.Getenv("GIT_BRANCH"); b != "" {
		return strings.TrimPrefix(b, "origin/")
	}
	if b := os.Getenv("BRANCH_NAME"); b != "" {
		return b
	}

	// Bitbucket Pipelines
	if b := os.Getenv("BITBUCKET_BRANCH"); b != "" {
		return b
	}

	// Azure DevOps
	if b := os.Getenv("BUILD_SOURCEBRANCH"); b != "" {
		return strings.TrimPrefix(b, "refs/heads/")
	}

	// Travis CI
	if b := os.Getenv("TRAVIS_BRANCH"); b != "" {
		return b
	}

	// CircleCI
	if b := os.Getenv("CIRCLE_BRANCH"); b != "" {
		return b
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
