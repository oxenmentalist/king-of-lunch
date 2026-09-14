package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/oxenmentalist/king-of-lunch/internal/native"
)

func main() {
	runtime.LockOSThread()
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "kol:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Println("Usage: kol [--] <filename>\nOpen a Markdown file in KING OF LUNCH.")
		return nil
	}
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	} else if len(args) > 0 && strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("unknown option %q; use -- before a filename starting with a dash", args[0])
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: kol [--] <filename>")
	}
	path, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	return native.Launch(path, applicationPath())
}
func applicationPath() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	dir := filepath.Dir(executable)
	if data, err := os.ReadFile(filepath.Join(dir, "kol.app-path")); err == nil {
		path := strings.TrimSpace(string(data))
		if filepath.IsAbs(path) && isApp(path) {
			return path
		}
	}
	path := filepath.Join(dir, "KING OF LUNCH.app")
	if isApp(path) {
		return path
	}
	return "" // NSWorkspace resolves the registered bundle identifier.
}
func isApp(path string) bool {
	info, err := os.Stat(filepath.Join(path, "Contents", "MacOS", "king-of-lunch"))
	return err == nil && info.Mode().IsRegular()
}
