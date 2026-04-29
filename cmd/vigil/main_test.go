package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

var testBin string

func TestMain(m *testing.M) {
	bin, dir, err := buildBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n", err)
		os.Exit(1)
	}
	testBin = bin
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestVersion(t *testing.T) {
	out, err := exec.Command(testBin, "--version").Output()
	if err != nil {
		t.Fatalf("vigil --version: %v", err)
	}

	got := strings.TrimSpace(string(out))
	if !strings.HasPrefix(got, "vigil ") {
		t.Errorf("expected output to start with 'vigil ', got %q", got)
	}
}

func TestNoArgs(t *testing.T) {
	cmd := exec.Command(testBin)
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err == nil {
		t.Fatal("expected non-zero exit code without arguments")
	}
}

func buildBinary() (string, string, error) {
	dir, err := os.MkdirTemp("", "vigil-test-*")
	if err != nil {
		return "", "", err
	}
	bin := dir + "/vigil"
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		return "", "", fmt.Errorf("%w\n%s", err, out)
	}
	return bin, dir, nil
}
