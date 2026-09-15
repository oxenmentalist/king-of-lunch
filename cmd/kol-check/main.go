// kol-check runs independent integration checks against the actual AppKit and
// WKWebView bridge. It is a developer diagnostic, not part of the shipped app.
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/oxenmentalist/king-of-lunch/internal/app"
	"github.com/oxenmentalist/king-of-lunch/internal/native"
)

type assertion struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail any    `json:"detail,omitempty"`
}

type sample struct {
	Name      string  `json:"name"`
	ElapsedMS float64 `json:"elapsed_ms"`
	Native    string  `json:"native,omitempty"`
}

type report struct {
	Passed   bool        `json:"passed"`
	Platform string      `json:"platform"`
	Go       string      `json:"go"`
	PID      int         `json:"pid"`
	Tests    []assertion `json:"tests"`
	Samples  []sample    `json:"samples"`
	Error    string      `json:"error,omitempty"`
}

type suite struct {
	app     *app.App
	backlog []native.Event
	token   uint64
	report  report
}

func (s *suite) check(name string, passed bool, detail any) {
	s.report.Tests = append(s.report.Tests, assertion{name, passed, detail})
	fmt.Fprintf(os.Stderr, "%s: %t\n", name, passed)
}

func (s *suite) wait(kind string, id, token uint64) native.Event {
	match := func(e native.Event) bool {
		return e.Kind == kind && (id == 0 || e.ID == id) && (token == 0 || e.Token == token)
	}
	for i, e := range s.backlog {
		if match(e) {
			s.backlog = append(s.backlog[:i], s.backlog[i+1:]...)
			return e
		}
	}
	timer := time.NewTimer(25 * time.Second)
	defer timer.Stop()
	for {
		select {
		case e := <-s.app.Events:
			if match(e) {
				return e
			}
			s.backlog = append(s.backlog, e)
		case <-timer.C:
			panic(fmt.Sprintf("timed out waiting for %s (document=%d token=%d); pending events=%+v", kind, id, token, s.backlog))
		}
	}
}

func (s *suite) evaluate(id uint64, expression string) any {
	s.token++
	native.Evaluate(id, s.token, expression)
	e := s.wait("evaluated", id, s.token)
	var value any
	if err := json.Unmarshal([]byte(e.Value), &value); err != nil {
		panic(fmt.Sprintf("invalid evaluation JSON %q: %v", e.Value, err))
	}
	return value
}

func (s *suite) number(id uint64, expression string) float64 {
	value := s.evaluate(id, expression)
	n, ok := value.(float64)
	if !ok {
		panic(fmt.Sprintf("expected number for %s, got %#v", expression, value))
	}
	return n
}

func (s *suite) eventually(id uint64, expression string) bool {
	deadline := time.Now().Add(5 * time.Second)
	for {
		if value, ok := s.evaluate(id, expression).(bool); ok && value {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func write(path, value string) {
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		panic(err)
	}
}

func (s *suite) open(path string) uint64 {
	s.app.Open(path)
	e := s.wait("opened", 0, 0)
	s.wait("loaded", e.ID, 0)
	return e.ID
}

func (s *suite) close(id uint64) {
	native.Action(id, "close")
	s.wait("closed", id, 0)
	s.wait("disposed", id, 0)
}

const fontSize = `parseFloat(getComputedStyle(document.body).fontSize)`

// These events exercise the app-installed isolated-world handlers inside the
// real WKWebView. They deliberately do not claim to synthesize physical input
// or verify WebKit's default handling of untrusted events.
func (s *suite) checkControls(id uint64, selector string) {
	value := s.evaluate(id, `(() => {
	  const region = Array.from(document.querySelectorAll('`+selector+`')).find(x => x.scrollWidth > x.clientWidth + 400);
	  if (!region) return {error:'no overflowing region'};
	  region.focus({preventScroll:true});
	  const target = region.querySelector('td, code') || region;
	  const key = (name, modifiers = {}) => {
	    const event = new KeyboardEvent('keydown', {key:name, bubbles:true, cancelable:true, ...modifiers});
	    const before = region.scrollLeft;
	    region.dispatchEvent(event);
	    return {before, after:region.scrollLeft, prevented:event.defaultPrevented};
	  };
	  const wheel = options => {
	    const event = new WheelEvent('wheel', {bubbles:true, cancelable:true, ...options});
	    const before = region.scrollLeft;
	    target.dispatchEvent(event);
	    return {before, after:region.scrollLeft, prevented:event.defaultPrevented};
	  };
	  const result = {};
	  region.scrollLeft = 0;
	  result.right = key('ArrowRight');
	  result.left = key('ArrowLeft');
	  const selection = getSelection();
	  selection.selectAllChildren(region);
	  const selected = selection.toString();
	  result.selectionLength = selected.length;
	  result.modifiers = [];
	  for (const modifier of ['shiftKey', 'metaKey', 'ctrlKey', 'altKey']) {
	    region.scrollLeft = 200;
	    result.modifiers.push({...key('ArrowRight', {[modifier]:true}), modifier, selectionPreserved:selection.toString() === selected});
	  }
	  selection.removeAllRanges();
	  region.scrollLeft = 200;
	  result.horizontalWheel = wheel({deltaX:120, deltaY:0});
	  region.scrollLeft = 200;
	  result.shiftWheel = wheel({deltaX:0, deltaY:80, shiftKey:true});
	  region.scrollLeft = 200;
	  result.verticalWheel = wheel({deltaX:0, deltaY:120});
	  region.scrollLeft = 0;
	  result.leftBoundaryWheel = wheel({deltaX:-120});
	  result.leftBoundaryKey = key('ArrowLeft');
	  region.scrollLeft = region.scrollWidth;
	  result.rightBoundaryWheel = wheel({deltaX:120});
	  result.rightBoundaryKey = key('ArrowRight');
	  region.scrollLeft = 0;
	  return result;
	})()`)
	cases, ok := value.(map[string]any)
	if !ok {
		panic(fmt.Sprintf("control-handler query returned %#v", value))
	}
	result := func(name string) map[string]any {
		m, _ := cases[name].(map[string]any)
		return m
	}
	right, left := result("right"), result("left")
	s.check(selector+" installed arrow handlers scroll 48 px and return", right["after"] == float64(48) && right["prevented"] == true && left["after"] == float64(0) && left["prevented"] == true, map[string]any{"right": right, "left": left})
	modifiers, _ := cases["modifiers"].([]any)
	selectionLength, _ := cases["selectionLength"].(float64)
	preserved := len(modifiers) == 4 && selectionLength > 0
	for _, entry := range modifiers {
		m, _ := entry.(map[string]any)
		preserved = preserved && m["after"] == float64(200) && m["prevented"] == false && m["selectionPreserved"] == true
	}
	s.check(selector+" modified arrows preserve selection and native handling", preserved, modifiers)
	horizontal, shift := result("horizontalWheel"), result("shiftWheel")
	s.check(selector+" installed horizontal wheel handler moves content", horizontal["after"] == float64(320) && horizontal["prevented"] == true, horizontal)
	s.check(selector+" installed Shift-wheel handler moves content", shift["after"] == float64(280) && shift["prevented"] == true, shift)
	vertical := result("verticalWheel")
	s.check(selector+" ordinary vertical wheel remains unhandled", vertical["after"] == float64(200) && vertical["prevented"] == false, vertical)
	boundary := make(map[string]any)
	boundaryPassed := true
	for _, name := range []string{"leftBoundaryWheel", "leftBoundaryKey", "rightBoundaryWheel", "rightBoundaryKey"} {
		m := result(name)
		boundary[name] = m
		boundaryPassed = boundaryPassed && m["before"] != nil && m["before"] == m["after"] && m["prevented"] == false
	}
	s.check(selector+" boundary input is not prevented", boundaryPassed, boundary)
}

// Navigation checks dispatch DOM events through the installed WKWebView handlers.
// Physical keyboard delivery and native default shortcuts are outside this check.
func (s *suite) checkVimNavigation(id uint64) {
	value := s.evaluate(id, `(() => {
	  const results = [];
	  const record = (name, passed, detail) => results.push({name, passed, detail});
	  const root = document.documentElement;
	  const max = root.scrollHeight - root.clientHeight;
	  const half = root.clientHeight / 2;
	  const middle = Math.min(2000, Math.floor(max / 2));
	  const near = (actual, expected) => Math.abs(actual - expected) <= 1;
	  const key = (name, options = {}, target = document.body, prevented = false) => {
	    const event = new KeyboardEvent('keydown', {key:name, bubbles:true, cancelable:true, ...options});
	    if (prevented) event.preventDefault();
	    target.dispatchEvent(event);
	    return {y:scrollY, prevented:event.defaultPrevented};
	  };
	  const reset = () => {
	    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
	    key('Escape');
	    window.scrollTo(0, middle);
	  };
	  record('vim fixture has room for half-page navigation', middle > half && max > middle + half, {max, half, middle});
	  reset();
	  const down = key('d', {ctrlKey:true});
	  const up = key('u', {ctrlKey:true});
	  record('Ctrl-d and Ctrl-u move half the viewport and return', near(down.y, middle + half) && near(up.y, middle) && down.prevented && up.prevented, {down, up, half});
	  window.scrollTo(0, max - 10);
	  const bottomClamp = key('d', {ctrlKey:true});
	  window.scrollTo(0, 10);
	  const topClamp = key('u', {ctrlKey:true});
	  record('Ctrl-d and Ctrl-u clamp at document boundaries', near(bottomClamp.y, max) && topClamp.y === 0, {bottomClamp, topClamp, max});
	  for (const shiftKey of [false, true]) {
	    reset();
	    const bottom = key('G', {shiftKey});
	    record('G reaches the bottom with shiftKey=' + shiftKey, near(bottom.y, max) && bottom.prevented, bottom);
	  }
	  reset();
	  const first = key('g');
	  const second = key('g');
	  record('single g waits and gg reaches the top', first.y === middle && second.y === 0 && second.prevented, {first, second});
	  reset();
	  key('g');
	  const repeats = [key('g', {repeat:true}), key('g', {repeat:true})];
	  record('holding g does not complete gg', repeats.every(x => x.y === middle), repeats);
	  const resets = [
	    ['unrelated key', () => key('x')],
	    ['modified key', () => key('g', {metaKey:true})],
	    ['window blur', () => window.dispatchEvent(new Event('blur'))],
	    ['pointerdown', () => document.body.dispatchEvent(new PointerEvent('pointerdown', {bubbles:true}))],
	    ['focus change', () => {
	      const button = document.createElement('button');
	      document.body.append(button);
	      button.focus({preventScroll:true});
	      button.blur();
	      button.remove();
	    }],
	    ['composition', () => key('g', {isComposing:true})],
	    ['already-handled event', () => key('g', {}, document.body, true)]
	  ];
	  for (const [name, interrupt] of resets) {
	    reset();
	    key('g');
	    interrupt();
	    const after = key('g');
	    record('gg prefix resets after ' + name, after.y === middle, after);
	  }
	  const shortcuts = [];
	  for (const modifier of ['metaKey', 'altKey', 'ctrlKey', 'shiftKey']) {
	    for (const name of ['g', 'G', 'u', 'd']) {
	      if (modifier === 'ctrlKey' && (name === 'u' || name === 'd')) continue;
	      if (modifier === 'shiftKey' && name === 'G') continue;
	      reset();
	      const event = key(name, {[modifier]:true});
	      shortcuts.push({name, modifier, ...event});
	    }
	  }
	  for (const modifier of ['metaKey', 'altKey', 'shiftKey']) {
	    for (const name of ['u', 'd']) {
	      reset();
	      shortcuts.push({name, modifier, ...key(name, {ctrlKey:true, [modifier]:true})});
	    }
	  }
	  record('modified shortcuts remain unhandled', shortcuts.every(x => x.y === middle && !x.prevented), shortcuts);
	  const ignored = [];
	  for (const [name, options] of [['u', {ctrlKey:true}], ['d', {ctrlKey:true}], ['G', {}], ['g', {}]]) {
	    reset();
	    const composing = key(name, {...options, isComposing:true});
	    const handled = key(name, options, document.body, true);
	    ignored.push({name, composing, handled});
	  }
	  record('composition and already-handled keys do not navigate', ignored.every(x => x.composing.y === middle && !x.composing.prevented && x.handled.y === middle), ignored);
	  const editors = [];
	  for (const tag of ['input', 'textarea', 'select', 'div']) {
	    reset();
	    const editor = document.createElement(tag);
	    if (tag === 'div') editor.contentEditable = 'true';
	    const target = tag === 'div' ? editor.appendChild(document.createElement('span')) : editor;
	    document.body.append(editor);
	    editor.focus({preventScroll:true});
	    window.scrollTo(0, middle);
	    const events = [key('u', {ctrlKey:true}, target), key('d', {ctrlKey:true}, target), key('G', {}, target), key('g', {}, target), key('g', {}, target)];
	    editors.push({tag, events});
	    editor.remove();
	  }
	  record('editable controls and contenteditable descendants retain their keys', editors.every(x => x.events.every(e => e.y === middle && !e.prevented)), editors);
	  for (const selector of ['pre', '.table-scroll']) {
	    reset();
	    const region = document.querySelector(selector);
	    region.focus({preventScroll:true});
	    region.scrollLeft = 100;
	    const beforeX = region.scrollLeft;
	    const event = key('d', {ctrlKey:true}, region);
	    record('vim navigation scrolls the document while ' + selector + ' is focused', near(event.y, middle + half) && region.scrollLeft === beforeX, {event, beforeX, afterX:region.scrollLeft});
	    region.scrollLeft = 0;
	  }
	  reset();
	  window.scrollTo(0, 0);
	  return results;
	})()`)
	results, ok := value.([]any)
	if !ok {
		panic(fmt.Sprintf("vim-navigation query returned %#v", value))
	}
	for _, entry := range results {
		result := entry.(map[string]any)
		s.check(result["name"].(string), result["passed"] == true, result["detail"])
	}
}

func (s *suite) checkSearch(id uint64) {
	value := s.evaluate(id, `(() => {
	  const results = [];
	  const record = (name, passed) => results.push({name, passed});
	  const key = (key, options = {}, target = document.activeElement) => {
	    const event = new KeyboardEvent('keydown', {key, bubbles:true, cancelable:true, ...options});
	    target.dispatchEvent(event);
	    return event.defaultPrevented;
	  };
	  const fixture = document.createElement('section');
	  fixture.innerHTML = '<p>kolneedle <strong>across</strong> markup</p><p>KOLNEEDLE across markup</p>';
	  document.querySelector('main').prepend(fixture);
	  getSelection().removeAllRanges();
	  document.activeElement.blur();
	  key(':'); key('x'); key('/');
	  record('search prefix resets after unrelated keys', !document.querySelector('.document-search'));
	  key(':'); key('/');
	  const input = document.querySelector('.document-search input');
	  const bar = input.closest('.document-search');
	  record('colon slash opens and focuses search', !bar.hidden && document.activeElement === input);
	  record('typing n in search keeps native input handling', !key('n'));
	  input.value = 'kolneedle across markup';
	  key('Enter');
	  const first = getSelection().anchorNode;
	  record('search selects text across inline markup and closes input', bar.hidden && getSelection().toString() === 'kolneedle across markup');
	  key('n');
	  const second = getSelection().anchorNode;
	  record('n advances case-insensitively', second !== first && getSelection().toString() === 'KOLNEEDLE across markup');
	  key('n');
	  record('forward search wraps', getSelection().anchorNode === first);
	  key('N', {shiftKey:true});
	  record('N searches backward and wraps', getSelection().anchorNode === second);
	  key(':'); key('/'); input.value = 'missing-kol-search-phrase'; key('Enter');
	  record('missing query shows feedback and stays editable', !bar.hidden && document.activeElement === input && bar.textContent.includes('No matches'));
	  key('Escape');
	  record('Escape closes search and restores document focus', bar.hidden && document.activeElement !== input);
	  record('Escape clears a missing query and disables repeat search', input.value === '' && !key('n') && !key('N', {shiftKey:true}) && bar.hidden);
	  key(':'); key('/');
	  record('search reopens empty after Escape', input.value === '');
	  input.value = 'unfinished query'; key('Escape');
	  key(':'); key('/');
	  record('Escape discards an unsubmitted query', input.value === '');
	  input.value = 'kolneedle across markup'; key('Enter');
	  record('search works again after clearing', bar.hidden && getSelection().toString().toLowerCase() === 'kolneedle across markup');
	  key('Escape');
	  record('Escape clears completed search while bar is closed', bar.hidden && input.value === '' && getSelection().toString() === '' && !key('n') && !key('N', {shiftKey:true}));
	  key(':'); key('/');
	  record('completed search reopens empty after Escape', input.value === '');
	  input.value = 'kolneedle'; key('Enter');
	  key(':'); key('/'); input.value = ''; key('Enter');
	  record('empty query clears search', bar.hidden && !key('n'));
	  fixture.remove();
	  getSelection().removeAllRanges();
	  window.scrollTo(0, 0);
	  return results;
	})()`)
	for _, entry := range value.([]any) {
		result := entry.(map[string]any)
		s.check(result["name"].(string), result["passed"] == true, nil)
	}
}

func (s *suite) run(dir, document, source string, started time.Time, iterations, idleSeconds int, stress, startupOnly, lifecycleOnly bool) {
	first := s.wait("opened", 0, 0)
	loaded := s.wait("loaded", first.ID, 0)
	id := first.ID
	s.report.Samples = append(s.report.Samples, sample{"initial-100KiB-document", float64(time.Since(started).Microseconds()) / 1000, loaded.Value})
	canonical, err := filepath.EvalSymlinks(document)
	if err != nil {
		panic(err)
	}
	s.check("initial document opened", first.Value == canonical, first.Value)

	value := s.evaluate(id, `(() => {
	  const firstCode = document.querySelector('pre code') || document.querySelector('pre');
	  const colors = firstCode ? Array.from(firstCode.querySelectorAll('span')).map(x => getComputedStyle(x).color) : [];
	  const overflow = selector => Array.from(document.querySelectorAll(selector)).some(el => {
	    for (let p = el; p && p !== document.body; p = p.parentElement) {
	      if (p.scrollWidth > p.clientWidth + 2 && /auto|scroll/.test(getComputedStyle(p).overflowX)) return true;
	    }
	    return false;
	  });
	  return {
	    heading: document.querySelector('h1')?.textContent,
	    tableCount: document.querySelectorAll('table').length,
	    tableAlignments: Array.from(document.querySelectorAll('thead th')).map(x => getComputedStyle(x).textAlign),
	    nestedList: !!document.querySelector('li li'),
	    taskList: document.querySelectorAll('input[type=checkbox]').length === 2,
	    quote: !!document.querySelector('blockquote'),
	    highlightColors: new Set(colors).size,
	    unknownCode: Array.from(document.querySelectorAll('pre')).some(x => x.textContent.includes('<unknown> & plain')),
	    verticalOverflow: document.documentElement.scrollHeight > innerHeight,
	    codeOverflow: overflow('pre'),
	    tableOverflow: overflow('table'),
	    fontSize: parseFloat(getComputedStyle(document.body).fontSize),
	    background: getComputedStyle(document.body).backgroundColor,
	    scriptSuppressed: typeof window.__kolPwn === 'undefined' && !document.querySelector('script, iframe'),
	    remoteImageURLs: Array.from(document.images).filter(x => /^https?:/.test(x.getAttribute('src') || '')).length,
	    forbiddenImagesLoaded: Array.from(document.images).filter(x => x.alt.startsWith('blocked')).some(x => x.naturalWidth > 0),
	    unsafeLinks: Array.from(document.links).some(x => /^(javascript|data|vbscript):/i.test(x.getAttribute('href') || ''))
	  };
	})()`)
	dom, ok := value.(map[string]any)
	if !ok {
		panic(fmt.Sprintf("DOM query returned %#v", value))
	}
	s.check("GFM heading, nested lists, task lists, quote and table", dom["heading"] == "Integration original" && dom["tableCount"] == float64(1) && dom["nestedList"] == true && dom["taskList"] == true && dom["quote"] == true, dom)
	s.check("recognized syntax has multiple token colors", dom["highlightColors"].(float64) >= 3, dom["highlightColors"])
	s.check("unknown code stays escaped and readable", dom["unknownCode"] == true, nil)
	s.check("vertical and horizontal overflow are reachable", dom["verticalOverflow"] == true && dom["codeOverflow"] == true && dom["tableOverflow"] == true, nil)
	alignments, _ := dom["tableAlignments"].([]any)
	s.check("table column alignment is honored", len(alignments) == 2 && alignments[0] == "left" && alignments[1] == "right", alignments)
	s.check("body starts at 16 pixels", dom["fontSize"] == float64(16), dom["fontSize"])
	s.check("unsafe HTML, URL schemes and remote images are suppressed", dom["scriptSuppressed"] == true && dom["remoteImageURLs"] == float64(0) && dom["unsafeLinks"] == false, nil)
	s.check("parent-directory and symlink images do not load", dom["forbiddenImagesLoaded"] == false, nil)
	s.check("permitted local image loads", s.eventually(id, `Array.from(document.images).some(x => x.alt === 'allowed' && x.naturalWidth > 0)`), nil)
	if idleSeconds > 0 {
		fmt.Fprintf(os.Stderr, "idle profiling window: pid=%d seconds=%d\n", os.Getpid(), idleSeconds)
		time.Sleep(time.Duration(idleSeconds) * time.Second)
	}
	if startupOnly {
		return
	}
	if lifecycleOnly {
		for i := 0; i < iterations; i++ {
			path := filepath.Join(dir, fmt.Sprintf("lifetime-%d.md", i))
			write(path, source)
			s.close(s.open(path))
		}
		s.check("closed document controllers are deallocated", true, iterations)
		return
	}
	for _, selector := range []string{".table-scroll", "pre"} {
		geometry := s.evaluate(id, `(() => {
		  const el = Array.from(document.querySelectorAll('`+selector+`')).find(x => x.scrollWidth > x.clientWidth + 2);
		  if (!el) return {found:false};
		  el.focus();
		  el.scrollLeft = 0;
		  const before = el.scrollLeft;
		  el.scrollLeft = 150;
		  return {found:true, width:el.clientWidth, scrollWidth:el.scrollWidth, before, after:el.scrollLeft, overflow:getComputedStyle(el).overflowX, focused:document.activeElement === el};
		})()`)
		geometryMap, _ := geometry.(map[string]any)
		after, _ := geometryMap["after"].(float64)
		s.check(selector+" permits horizontal scrolling and focus", geometryMap["found"] == true && geometryMap["focused"] == true && after > 0, geometry)
		s.checkControls(id, selector)
	}
	s.checkVimNavigation(id)
	s.checkSearch(id)
	s.app.Open(filepath.Join(filepath.Dir(document), ".", filepath.Base(document)))
	s.evaluate(id, `true`)
	duplicate := false
	for _, pending := range s.backlog {
		duplicate = duplicate || pending.Kind == "opened"
	}
	s.check("reopening normalized path keeps the existing window", !duplicate, nil)
	alias := filepath.Join(dir, "document-alias.md")
	if err := os.Symlink(document, alias); err != nil {
		panic(err)
	}
	s.app.Open(alias)
	s.evaluate(id, `true`)
	for _, pending := range s.backlog {
		duplicate = duplicate || pending.Kind == "opened"
	}
	s.check("reopening symlink alias keeps the existing window", !duplicate, nil)

	s.evaluate(id, `document.querySelector('a[href="#bottom"]').click(); true`)
	s.check("internal anchor navigation scrolls within the document", s.eventually(id, `scrollY > 1000`), nil)
	s.evaluate(id, `window.scrollTo(0,0); true`)
	linked := filepath.Join(dir, "linked.md")
	write(linked, "# Linked Markdown\n\n"+strings.Repeat("A paragraph keeps the linked target below the initial viewport.\n\n", 120)+"## Linked target café 日本語\n\nNative cross-document anchor navigation worked.\n")
	s.evaluate(id, `Array.from(document.links).find(x => x.textContent === 'local document').click(); true`)
	linkOpened := s.wait("opened", 0, 0)
	s.wait("loaded", linkOpened.ID, 0)
	s.check("local Markdown link opens another document", s.evaluate(linkOpened.ID, `document.querySelector('h1').textContent`) == "Linked Markdown", nil)
	s.check("Unicode cross-document anchor reaches a newly opened document", s.eventually(linkOpened.ID, `scrollY > 1000 && document.getElementById('linked-target-café-日本語').getBoundingClientRect().top < innerHeight`), nil)
	s.evaluate(linkOpened.ID, `window.scrollTo(0,0); true`)
	s.eventually(linkOpened.ID, `scrollY === 0`)
	s.evaluate(id, `Array.from(document.links).find(x => x.textContent === 'local document').click(); true`)
	s.check("Unicode cross-document anchor reaches an already-open document", s.eventually(linkOpened.ID, `scrollY > 1000 && document.getElementById('linked-target-café-日本語').getBoundingClientRect().top < innerHeight`), nil)
	s.close(linkOpened.ID)

	native.Resize(id, 420, 360)
	s.check("narrow window retains horizontal overflow", s.eventually(id, `Array.from(document.querySelectorAll('pre')).some(x => x.scrollWidth > x.clientWidth + 2 || x.parentElement.scrollWidth > x.parentElement.clientWidth + 2)`), nil)
	native.Resize(id, 960, 720)
	native.Action(id, "zoomIn")
	s.check("zoom increase is font size change", s.eventually(id, fontSize+` === 18`), nil)
	for i := 0; i < 20; i++ {
		native.Action(id, "zoomIn")
	}
	s.check("zoom upper bound is 32 pixels", s.eventually(id, fontSize+` === 32`), nil)
	for i := 0; i < 20; i++ {
		native.Action(id, "zoomOut")
	}
	s.check("zoom lower bound is 10 pixels", s.eventually(id, fontSize+` === 10`), nil)
	native.Action(id, "zoomReset")
	s.check("zoom reset is 16 pixels", s.eventually(id, fontSize+` === 16`), nil)
	native.Action(id, "zoomIn")
	s.eventually(id, fontSize+` === 18`)
	s.evaluate(id, `window.scrollTo(0, 550); true`)
	s.eventually(id, `scrollY > 300`)
	beforeScroll := s.number(id, `scrollY`)
	updated := strings.Replace(source, "Integration original", "Integration reloaded", 1)
	write(document, updated)
	time.Sleep(300 * time.Millisecond)
	s.check("file is unchanged on screen until explicit reload", s.evaluate(id, `document.querySelector('h1').textContent`) == "Integration original", nil)
	reloadStart := time.Now()
	native.Action(id, "reload")
	loaded = s.wait("loaded", id, 0)
	s.report.Samples = append(s.report.Samples, sample{"reload-100KiB-document", float64(time.Since(reloadStart).Microseconds()) / 1000, loaded.Value})
	s.check("reload reads new disk contents", s.evaluate(id, `document.querySelector('h1').textContent`) == "Integration reloaded", nil)
	s.check("reload retains per-window zoom", s.number(id, fontSize) == 18, nil)
	afterScroll := s.number(id, `scrollY`)
	s.check("reload restores scroll", beforeScroll > 300 && afterScroll >= beforeScroll-32 && afterScroll <= beforeScroll+32, map[string]float64{"before": beforeScroll, "after": afterScroll})

	write(document, string([]byte{0xff, 0xfe}))
	native.Action(id, "reload")
	e := s.wait("error", id, 0)
	s.check("invalid encoding reports error and retains content", e.Value != "" && s.evaluate(id, `document.querySelector('h1').textContent`) == "Integration reloaded", e.Value)
	native.Action(id, "dismissError")
	if err := os.Remove(document); err != nil {
		panic(err)
	}
	native.Action(id, "reload")
	e = s.wait("error", id, 0)
	s.check("deleted file reports error and retains content", e.Value != "" && s.evaluate(id, `document.querySelector('h1').textContent`) == "Integration reloaded", e.Value)
	native.Action(id, "dismissError")
	write(document, updated)
	native.Action(id, "reload")
	s.wait("loaded", id, 0)

	empty := filepath.Join(dir, "empty.md")
	write(empty, "")
	second := s.open(empty)
	s.check("empty document is valid and independently zoomed", s.evaluate(second, `document.body.innerText.trim()`) == "" && s.number(second, fontSize) == 16 && s.number(id, fontSize) == 18, nil)
	s.close(second)
	write(empty, "\ufeff# BOM and CRLF\r\n\r\nUnicode 日本語 café 👑\r\n")
	second = s.open(empty)
	s.check("BOM, CRLF and Unicode render", s.evaluate(second, `document.querySelector('h1').textContent`) == "BOM and CRLF" && s.evaluate(second, `document.body.innerText.includes('日本語 café 👑')`) == true, nil)
	s.close(second)

	for i := 0; i < iterations; i++ {
		path := filepath.Join(dir, fmt.Sprintf("repeat-%d.md", i))
		write(path, source)
		start := time.Now()
		s.app.Open(path)
		opened := s.wait("opened", 0, 0)
		loaded = s.wait("loaded", opened.ID, 0)
		s.report.Samples = append(s.report.Samples, sample{"warm-open-100KiB-document", float64(time.Since(start).Microseconds()) / 1000, loaded.Value})
		s.close(opened.ID)
	}
	s.check("repeated document open and close completes", true, iterations)
	for i := 0; i < 8; i++ {
		native.Action(id, "reload")
	}
	s.wait("loaded", id, 0)
	s.check("rapid reload leaves latest contents readable", s.eventually(id, `document.querySelector('h1').textContent === 'Integration reloaded'`), nil)

	if stress {
		for _, size := range []int{1024 * 1024, 10 * 1024 * 1024} {
			path := filepath.Join(dir, fmt.Sprintf("stress-%d.md", size))
			paragraph := "A plain paragraph with **bold** text and `code`.\n\n"
			write(path, strings.Repeat(paragraph, size/len(paragraph)+1)[:size])
			start := time.Now()
			s.app.Open(path)
			opened := s.wait("opened", 0, 0)
			loaded = s.wait("loaded", opened.ID, 0)
			s.report.Samples = append(s.report.Samples, sample{fmt.Sprintf("stress-open-%d-bytes", size), float64(time.Since(start).Microseconds()) / 1000, loaded.Value})
			s.check(fmt.Sprintf("%d-byte document renders", size), s.number(opened.ID, `document.documentElement.scrollHeight`) > 1000, nil)
			s.close(opened.ID)
		}
	}
}

func fixtures() (dir, document, source string) {
	var err error
	dir, err = os.MkdirTemp("", "kol-native-check-")
	if err != nil {
		panic(err)
	}
	documents := filepath.Join(dir, "documents")
	images := filepath.Join(documents, "images")
	if err = os.MkdirAll(images, 0700); err != nil {
		panic(err)
	}
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jBhcAAAAASUVORK5CYII=")
	if err != nil {
		panic(err)
	}
	write(filepath.Join(images, "allowed.png"), string(png))
	// An existing file outside the document root must remain inaccessible.
	outside := filepath.Join(dir, "outside.png")
	write(outside, string(png))
	if err = os.Symlink(outside, filepath.Join(images, "escape.png")); err != nil {
		panic(err)
	}
	source = "# Integration original\n\n**Bold**, *emphasis*, ~~strike~~, `inline`, Unicode 日本語 café 👑.\n\n- parent\n  - child\n- [x] done\n- [ ] pending\n\n> Quoted text\n\n" +
		"```go\npackage main\n// A comment.\nfunc main() { println(\"hello\", 42) }\n```\n\n```unknown-language\n<unknown> & plain\n```\n\n```\n" + strings.Repeat("long_unbroken_text_", 100) + "\n```\n\n" +
		"| Left | Right |\n|:--|--:|\n| one | " + strings.Repeat("wide_table_cell_", 100) + " |\n\n" +
		"![allowed](images/allowed.png)\n![blocked symlink](images/escape.png)\n![blocked parent](../outside.png)\n![blocked remote](https://127.0.0.1:9/private.png)\n\n" +
		"[unsafe](javascript:alert(1)), [jump](#bottom), [local document](../linked.md#linked-target-caf%C3%A9-%E6%97%A5%E6%9C%AC%E8%AA%9E).\n\n<script>window.__kolPwn = true;</script>\n<iframe src=\"https://127.0.0.1:9/\"></iframe>\n\n"
	paragraph := "## Paragraph\n\nA readable paragraph with **bold** and `inline code` for vertical scroll and representative document layout.\n\n"
	source += strings.Repeat(paragraph, (100*1024-len(source))/len(paragraph)+1)
	source += "\n## Bottom\n\nEnd of the document.\n"
	document = filepath.Join(documents, "integration.md")
	write(document, source)
	return dir, document, source
}

func main() {
	stress := flag.Bool("stress", false, "also render 1 MiB and 10 MiB documents")
	startupOnly := flag.Bool("startup-only", false, "check initial rendering and exit without exercising controls")
	lifecycleOnly := flag.Bool("lifecycle-only", false, "check initial rendering and repeated controller deallocation without the full suite")
	iterations := flag.Int("iterations", 5, "number of repeated 100 KiB open/close samples")
	idleSeconds := flag.Int("idle-seconds", 0, "pause after initial rendering for external process profiling (maximum 60)")
	flag.Parse()
	if *iterations < 0 || *iterations > 30 {
		fmt.Fprintln(os.Stderr, "iterations must be between 0 and 30")
		os.Exit(2)
	}
	if *idleSeconds < 0 || *idleSeconds > 60 {
		fmt.Fprintln(os.Stderr, "idle-seconds must be between 0 and 60")
		os.Exit(2)
	}
	if *startupOnly && *lifecycleOnly {
		fmt.Fprintln(os.Stderr, "choose only one of startup-only and lifecycle-only")
		os.Exit(2)
	}
	runtime.LockOSThread()
	os.Setenv("KOL_CHECK", "1")
	dir, document, source := fixtures()
	defer os.RemoveAll(dir)
	a := app.New()
	s := &suite{app: a, report: report{Platform: runtime.GOOS + "/" + runtime.GOARCH, Go: runtime.Version(), PID: os.Getpid()}}
	done := make(chan report, 1)
	started := time.Now()
	go func() {
		defer func() {
			if err := recover(); err != nil {
				s.report.Error = fmt.Sprint(err)
			}
			s.report.Passed = s.report.Error == ""
			for _, test := range s.report.Tests {
				if !test.Passed {
					s.report.Passed = false
				}
			}
			done <- s.report
			native.Stop()
		}()
		s.run(dir, document, source, started, *iterations, *idleSeconds, *stress, *startupOnly, *lifecycleOnly)
	}()
	a.Run([]string{document})
	result := <-done
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !result.Passed {
		os.Exit(1)
	}
}
