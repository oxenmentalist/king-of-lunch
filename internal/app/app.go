// Package app owns document state and rendering; the native package owns views.
package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/oxenmentalist/king-of-lunch/internal/native"
	"github.com/oxenmentalist/king-of-lunch/internal/render"
)

type document struct {
	id, generation uint64
	path           string
	font           int
	rendering      bool
}
type App struct {
	mu        sync.Mutex
	documents map[uint64]*document
	paths     map[string]uint64
	next      uint64
	workers   chan struct{}
	// Events supports the independent diagnostic runner. Production never reads
	// it, so reporting must never block the native main thread.
	Events chan native.Event
}

func New() *App {
	return &App{documents: make(map[uint64]*document), paths: make(map[string]uint64), Events: make(chan native.Event, 1024), workers: make(chan struct{}, 2)}
}
func (a *App) Run(paths []string) {
	native.Handler = func(e native.Event) {
		switch e.Kind {
		case "ready":
			for _, path := range paths {
				if !strings.HasPrefix(path, "-psn_") {
					a.Open(path)
				}
			}
		case "open":
			a.Open(e.Value)
		case "open-link":
			var link struct {
				Path     string  `json:"path"`
				Fragment *string `json:"fragment"`
			}
			if json.Unmarshal([]byte(e.Value), &link) == nil && link.Path != "" {
				a.open(link.Path, link.Fragment)
			}
		case "reload":
			a.reload(e.ID)
		case "zoom":
			delta, _ := strconv.Atoi(e.Value)
			a.zoom(e.ID, delta)
		case "closed":
			a.mu.Lock()
			if d := a.documents[e.ID]; d != nil {
				delete(a.paths, d.path)
				delete(a.documents, e.ID)
			}
			a.mu.Unlock()
		}
		select {
		case a.Events <- e:
		default:
		}
	}
	native.Run()
}

func (a *App) Open(path string) {
	a.open(path, nil)
}

func (a *App) open(path string, fragment *string) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		native.Error(0, 0, err.Error())
		return
	}
	// Resolve aliases so opening the same document from CLI and Finder focuses
	// one window. Keep an absolute path if the file disappeared before delivery.
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = resolved
	}
	a.mu.Lock()
	if id, ok := a.paths[absolute]; ok {
		native.Show(id, absolute)
		if fragment != nil {
			native.Anchor(id, *fragment)
		}
		a.mu.Unlock()
		return
	}
	a.next++
	id := a.next
	a.documents[id] = &document{id: id, path: absolute, font: 16}
	a.paths[absolute] = id
	native.Show(id, absolute)
	if fragment != nil {
		native.Anchor(id, *fragment)
	}
	a.mu.Unlock()
	a.reload(id)
}
func (a *App) reload(id uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	d := a.documents[id]
	if d == nil {
		return
	}
	d.generation++
	native.Begin(id, d.generation)
	if d.rendering {
		return
	}
	d.rendering = true
	go a.renderLoop(id)
}
func (a *App) renderLoop(id uint64) {
	for {
		a.workers <- struct{}{}
		a.mu.Lock()
		d := a.documents[id]
		if d == nil {
			a.mu.Unlock()
			<-a.workers
			return
		}
		generation, path := d.generation, d.path
		a.mu.Unlock()
		html, err := load(path)
		<-a.workers
		a.mu.Lock()
		current := a.documents[id]
		if current == nil {
			a.mu.Unlock()
			return
		}
		if current.generation != generation {
			a.mu.Unlock()
			continue
		}
		current.rendering = false
		if err != nil {
			native.Error(id, generation, err.Error())
		} else {
			native.Content(id, generation, html)
		}
		a.mu.Unlock()
		return
	}
}
func load(path string) (string, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", filepath.Base(path))
	}
	// Bound allocation for an accidental huge/binary input while allowing large
	// Markdown documents well beyond the validated stress fixtures.
	if info.Size() > 64<<20 {
		return "", fmt.Errorf("file exceeds the 64 MiB document limit")
	}
	data, err := io.ReadAll(io.LimitReader(f, (64<<20)+1))
	if err != nil {
		return "", err
	}
	if len(data) > 64<<20 {
		return "", fmt.Errorf("file exceeds the 64 MiB document limit")
	}
	return render.Render(data, path)
}
func (a *App) zoom(id uint64, delta int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	d := a.documents[id]
	if d == nil {
		return
	}
	if delta == 0 {
		d.font = 16
	} else {
		d.font += delta
	}
	if d.font < 10 {
		d.font = 10
	}
	if d.font > 32 {
		d.font = 32
	}
	native.Zoom(id, d.font)
}
