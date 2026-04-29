package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	bin, err := buildBinary(t)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		t.Fatalf("vigil --version: %v", err)
	}

	got := strings.TrimSpace(string(out))
	if !strings.HasPrefix(got, "vigil ") {
		t.Errorf("expected output to start with 'vigil ', got %q", got)
	}
}

func TestNoArgs(t *testing.T) {
	bin, err := buildBinary(t)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	cmd := exec.Command(bin)
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code without arguments")
	}
}

func buildBinary(t *testing.T) (string, error) {
	t.Helper()
	bin := t.TempDir() + "/vigil"
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%w\n%s", err, out)
	}
	return bin, nil
}
