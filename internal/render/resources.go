package render

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	maxImageBytes    = 8 << 20
	maxTotalImages   = 24 << 20
	maxImagePixels   = 24_000_000
	maxHighlightSize = 64 << 10
)

type localImage struct {
	data, reason  string
	width, height int
}

type documentRenderer struct {
	path               string
	root               *os.Root
	rootAttempted      bool
	images             map[string]localImage
	imageBytes         int64
	highlightRemaining time.Duration
}

func (r *documentRenderer) close() {
	if r.root != nil {
		_ = r.root.Close()
	}
}

func parsedDestination(raw []byte) (*url.URL, bool) {
	destination := unescape(raw)
	// Backslashes have URL-parser-specific behavior. A deliberately small
	// allowlist avoids interpreting them differently in Go and WebKit.
	if strings.ContainsAny(destination, "\\\x00\r\n\t") {
		return nil, false
	}
	u, err := url.Parse(destination)
	if err != nil || u.User != nil || u.Opaque != "" {
		return nil, false
	}
	if strings.ContainsAny(u.Path+u.Fragment, "\\\x00\r\n\t") {
		return nil, false
	}
	return u, true
}

func (r *documentRenderer) linkTarget(raw []byte) (string, bool) {
	u, ok := parsedDestination(raw)
	if !ok {
		return "", false
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme == "http" || u.Scheme == "https" {
		if u.Hostname() == "" {
			return "", false
		}
		return u.String(), true
	}
	if u.Host != "" || (u.Scheme != "" && u.Scheme != "file") || u.RawQuery != "" {
		return "", false
	}
	if u.Path == "" && u.Scheme == "" {
		return u.String(), true // A fragment or empty same-document link.
	}
	ext := strings.ToLower(filepath.Ext(u.Path))
	if ext != ".md" && ext != ".markdown" {
		return "", false
	}
	path := u.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(r.path), path)
	}
	// WebKit rejects navigation from the private document origin to file://
	// before the delegate can handle it. This private URL is only a transport
	// to the native navigation delegate; it never loads a resource itself.
	return (&url.URL{Scheme: "kol-file", Host: "document", Path: filepath.Clean(path), Fragment: u.Fragment}).String(), true
}

func (r *documentRenderer) localImage(raw []byte) localImage {
	key := string(raw)
	if img, ok := r.images[key]; ok {
		return img
	}
	img := r.readImage(raw)
	r.images[key] = img
	return img
}

func (r *documentRenderer) readImage(raw []byte) localImage {
	blocked := func(reason string) localImage { return localImage{reason: reason} }
	u, ok := parsedDestination(raw)
	if !ok || u.Scheme != "" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || u.Path == "" {
		return blocked("only images in the document folder are supported")
	}
	if !r.rootAttempted {
		r.rootAttempted = true
		r.root, _ = os.OpenRoot(filepath.Dir(r.path))
	}
	if r.root == nil {
		return blocked("the document folder cannot be read")
	}
	path := u.Path
	if filepath.IsAbs(path) {
		var err error
		path, err = filepath.Rel(filepath.Dir(r.path), path)
		if err != nil {
			return blocked("the image is outside the document folder")
		}
	}
	// os.Root applies the containment check during open, including symlinks;
	// a separate EvalSymlinks/check/open sequence would have a race window.
	f, err := r.root.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return blocked("the image is missing or outside the document folder")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return blocked("the image is not a readable regular file")
	}
	if info.Size() > maxImageBytes || r.imageBytes+info.Size() > maxTotalImages {
		return blocked("the image size limit was reached")
	}
	b, err := io.ReadAll(io.LimitReader(f, maxImageBytes+1))
	if err != nil || len(b) > maxImageBytes || r.imageBytes+int64(len(b)) > maxTotalImages {
		return blocked("the image cannot be read within the size limit")
	}
	config, kind, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxImagePixels {
		return blocked("use a PNG, JPEG, or GIF image of at most 24 megapixels")
	}
	mime := map[string]string{"png": "image/png", "jpeg": "image/jpeg", "gif": "image/gif"}[kind]
	if mime == "" {
		return blocked("use a PNG, JPEG, or GIF image")
	}
	r.imageBytes += int64(len(b))
	return localImage{data: "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b), width: config.Width, height: config.Height}
}
