package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"testing"
)

// TestHelperProcess is not a real test: runMain re-executes the test binary
// into it to run main() with given arguments, so exit codes are observable.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("SANSHAIN_GO_HELPER_PROCESS") != "1" {
		t.Skip("helper process only")
	}
	args := []string{"sanshain-go"}
	for i, a := range os.Args {
		if a == "--" {
			args = append(args, os.Args[i+1:]...)
			break
		}
	}
	os.Args = args
	main()
	os.Exit(0)
}

// runMain runs main() in a subprocess with args and extra environment and
// returns its stdout, stderr and exit code.
func runMain(t *testing.T, env []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestHelperProcess$", "--"}, args...)...)
	cmd.Env = append(append(os.Environ(),
		"SANSHAIN_GO_HELPER_PROCESS=1",
		"SANSHAIN_URL=", "SANSHAIN_BEST_EFFORT=", "SANSHAIN_TRUNK=", "SANSHAIN_TAG=", "SANSHAIN_GA=", "SANSHAIN_TOKEN=",
	), env...)
	cmd.Dir = t.TempDir()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("failed to run helper process: %v", err)
	}
	return stdout.String(), stderr.String(), code
}
