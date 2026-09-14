package app

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLoadDocument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(path, []byte("\xef\xbb\xbf# Lunch\r\n\r\nHello café."), 0600); err != nil {
		t.Fatal(err)
	}
	html, err := load(path)
	if err != nil || !strings.Contains(html, "Hello café.") || !strings.Contains(html, "<h1") {
		t.Fatalf("load: %v, %s", err, html)
	}
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := load(path); err != nil {
		t.Fatal("empty file:", err)
	}
	if err := os.WriteFile(path, []byte{0xff}, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := load(path); err == nil {
		t.Fatal("invalid UTF8 accepted")
	}
	if _, err := load(dir); err == nil {
		t.Fatal("directory accepted")
	}
	if _, err := load(filepath.Join(dir, "missing.md")); err == nil {
		t.Fatal("missing file accepted")
	}
}

func TestLoadRejectsFIFOWithoutBlocking(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pipe.md")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := load(path); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("blocked on FIFO")
	}
}

func TestLoadRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.md")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate((64 << 20) + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err = load(path); err == nil || !strings.Contains(err.Error(), "64 MiB") {
		t.Fatalf("oversized input: %v", err)
	}
}
