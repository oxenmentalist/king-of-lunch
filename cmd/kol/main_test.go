package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestInvalidArgumentsDoNotLaunch(t *testing.T) {
	for _, args := range [][]string{nil, {"one", "two"}, {"--bad"}, {"--"}} {
		if err := run(args); err == nil {
			t.Fatalf("accepted %q", args)
		}
	}
	if err := run([]string{"--help"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{t.TempDir()}); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("directory: %v", err)
	}
	if err := run([]string{filepath.Join(t.TempDir(), "missing.md")}); err == nil {
		t.Fatal("missing path accepted")
	}
}

func TestFIFOArgumentReturns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "named pipe.md")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- run([]string{path}) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("blocked on FIFO")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}
