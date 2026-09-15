// Package native is the narrow, copied-data boundary to AppKit and WebKit.
package native

/*
#cgo CFLAGS: -mmacosx-version-min=13.0 -x objective-c -fobjc-arc
#cgo LDFLAGS: -mmacosx-version-min=13.0 -framework AppKit -framework WebKit
#include <stdlib.h>
#include "native.h"
*/
import "C"

import (
	_ "embed"
	"errors"
	"unsafe"
)

//go:embed controls.js
var controlScript string

type Event struct {
	Kind  string
	ID    uint64
	Value string
	Token uint64
}

// Handler is set once before Run and is called on the native main thread.
// It must not block waiting for UI work.
var Handler func(Event)

//export goNativeEvent
func goNativeEvent(kind *C.char, id C.uint64_t, value *C.char, token C.uint64_t) {
	if Handler != nil {
		Handler(Event{C.GoString(kind), uint64(id), C.GoString(value), uint64(token)})
	}
}

func withString(s string, f func(*C.char)) { p := C.CString(s); defer C.free(unsafe.Pointer(p)); f(p) }
func Run()                                 { withString(controlScript, func(p *C.char) { C.kol_run(p) }) }
func Show(id uint64, path string) {
	withString(path, func(p *C.char) { C.kol_show(C.uint64_t(id), p) })
}
func Anchor(id uint64, fragment string) {
	withString(fragment, func(p *C.char) { C.kol_anchor(C.uint64_t(id), p) })
}
func Begin(id, generation uint64) { C.kol_begin(C.uint64_t(id), C.uint64_t(generation)) }
func Content(id, generation uint64, html string) {
	withString(html, func(p *C.char) { C.kol_content(C.uint64_t(id), C.uint64_t(generation), p) })
}
func Error(id, generation uint64, message string) {
	withString(message, func(p *C.char) { C.kol_error(C.uint64_t(id), C.uint64_t(generation), p) })
}
func Zoom(id uint64, size int) { C.kol_zoom(C.uint64_t(id), C.int(size)) }
func Launch(path, application string) error {
	var result *C.char
	withString(path, func(p *C.char) { withString(application, func(a *C.char) { result = C.kol_launch(p, a) }) })
	if result == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(result))
	return errors.New(C.GoString(result))
}

// These internal hooks are used by the separate diagnostic executable. They
// dispatch to the same views/actions as the application, never to page scripts.
func Evaluate(id, token uint64, script string) {
	withString(script, func(p *C.char) { C.kol_evaluate(C.uint64_t(id), C.uint64_t(token), p) })
}
func Action(id uint64, action string) {
	withString(action, func(p *C.char) { C.kol_action(C.uint64_t(id), p) })
}

// PrintPDF runs the document's print operation into a PDF file without a
// panel. A "printed" event reports whether the operation succeeded.
func PrintPDF(id uint64, path string) {
	withString(path, func(p *C.char) { C.kol_print_pdf(C.uint64_t(id), p) })
}
func Resize(id uint64, width, height int) { C.kol_resize(C.uint64_t(id), C.int(width), C.int(height)) }
func Stop()                               { C.kol_stop() }
