package git

import (
	"os"
	"testing"
)

var ciEnvVars = []string{
	"SANSHAIN_BRANCH", "GITHUB_HEAD_REF", "GITHUB_REF_NAME",
	"CI_COMMIT_BRANCH", "CI_MERGE_REQUEST_SOURCE_BRANCH_NAME",
	"GIT_BRANCH", "BRANCH_NAME", "BITBUCKET_BRANCH",
	"BUILD_SOURCEBRANCH", "TRAVIS_BRANCH", "CIRCLE_BRANCH",
}

func clearCIEnvVars() {
	for _, v := range ciEnvVars {
		os.Setenv(v, "")
	}
}

func backupAndRestoreCIEnvVars(t *testing.T) {
	originals := make(map[string]string)
	for _, v := range ciEnvVars {
		originals[v] = os.Getenv(v)
	}
	t.Cleanup(func() {
		for _, v := range ciEnvVars {
			os.Setenv(v, originals[v])
		}
	})
}

func TestGetCurrentBranch(t *testing.T) {
	backupAndRestoreCIEnvVars(t)

	t.Run("SANSHAIN_BRANCH", func(t *testing.T) {
		clearCIEnvVars()
		os.Setenv("SANSHAIN_BRANCH", "feature-x")
		if b := GetCurrentBranch(); b != "feature-x" {
			t.Errorf("expected feature-x, got %s", b)
		}
	})

	t.Run("GITHUB_HEAD_REF", func(t *testing.T) {
		clearCIEnvVars()
		os.Setenv("GITHUB_HEAD_REF", "pr-branch")
		if b := GetCurrentBranch(); b != "pr-branch" {
			t.Errorf("expected pr-branch, got %s", b)
		}
	})

	t.Run("GITHUB_REF_NAME", func(t *testing.T) {
		clearCIEnvVars()
		os.Setenv("GITHUB_REF_NAME", "main")
		if b := GetCurrentBranch(); b != "main" {
			t.Errorf("expected main, got %s", b)
		}
	})

	t.Run("CI_COMMIT_BRANCH", func(t *testing.T) {
		clearCIEnvVars()
		os.Setenv("CI_COMMIT_BRANCH", "gitlab-branch")
		if b := GetCurrentBranch(); b != "gitlab-branch" {
			t.Errorf("expected gitlab-branch, got %s", b)
		}
	})

	t.Run("GIT_BRANCH with origin prefix", func(t *testing.T) {
		clearCIEnvVars()
		os.Setenv("GIT_BRANCH", "origin/develop")
		if b := GetCurrentBranch(); b != "develop" {
			t.Errorf("expected develop, got %s", b)
		}
	})

	t.Run("BITBUCKET_BRANCH", func(t *testing.T) {
		clearCIEnvVars()
		os.Setenv("BITBUCKET_BRANCH", "bb-branch")
		if b := GetCurrentBranch(); b != "bb-branch" {
			t.Errorf("expected bb-branch, got %s", b)
		}
	})

	t.Run("BUILD_SOURCEBRANCH with refs/heads prefix", func(t *testing.T) {
		clearCIEnvVars()
		os.Setenv("BUILD_SOURCEBRANCH", "refs/heads/azure-branch")
		if b := GetCurrentBranch(); b != "azure-branch" {
			t.Errorf("expected azure-branch, got %s", b)
		}
	})

	t.Run("TRAVIS_BRANCH", func(t *testing.T) {
		clearCIEnvVars()
		os.Setenv("TRAVIS_BRANCH", "travis-branch")
		if b := GetCurrentBranch(); b != "travis-branch" {
			t.Errorf("expected travis-branch, got %s", b)
		}
	})

	t.Run("CIRCLE_BRANCH", func(t *testing.T) {
		clearCIEnvVars()
		os.Setenv("CIRCLE_BRANCH", "circle-branch")
		if b := GetCurrentBranch(); b != "circle-branch" {
			t.Errorf("expected circle-branch, got %s", b)
		}
	})

	t.Run("Fallback to git or main", func(t *testing.T) {
		clearCIEnvVars()
		b := GetCurrentBranch()
		if b == "" {
			t.Error("expected non-empty branch")
		}
	})
}
