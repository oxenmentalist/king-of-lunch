// App-owned controls execute only in an isolated WKContentWorld. Markdown
// cannot supply scripts or reach this world. These handlers add Vim document
// navigation and predictable horizontal overflow controls.
(() => {
  let pendingG = false;
  let pendingColon = false;
  const resetSequence = () => { pendingG = false; pendingColon = false; };
  let query = '';
  let searchBar;
  let searchInput;
  let searchStatus;
  let savedSelection;

  function closeSearch() {
    searchInput.blur();
    searchBar.hidden = true;
    const selection = getSelection();
    selection.removeAllRanges();
    if (savedSelection) selection.addRange(savedSelection);
  }

  function findMatch(backward = false) {
    if (!query) return;
    // WebKit searches rendered text across inline markup, selects the result,
    // and scrolls both the document and overflowing code/table containers.
    const found = window.find(query, false, backward, true, false, false, false);
    if (!found) {
      openSearch();
      searchStatus.textContent = 'No matches';
    }
  }

  function openSearch() {
    if (!searchBar) {
      searchBar = document.createElement('div');
      searchBar.className = 'document-search';
      searchBar.setAttribute('role', 'search');
      const label = document.createElement('label');
      label.textContent = ':/';
      searchInput = document.createElement('input');
      searchInput.type = 'text';
      searchInput.setAttribute('aria-label', 'Search document');
      searchInput.placeholder = 'Search';
      searchInput.autocomplete = 'off';
      searchInput.spellcheck = false;
      label.append(searchInput);
      searchStatus = document.createElement('span');
      searchStatus.setAttribute('role', 'status');
      searchBar.append(label, searchStatus);
      document.body.append(searchBar);
      searchInput.addEventListener('input', () => { searchStatus.textContent = ''; });
      searchInput.addEventListener('keydown', event => {
        if (event.isComposing || event.metaKey || event.ctrlKey || event.altKey) return;
        if (event.key === 'Escape') {
          event.preventDefault();
          closeSearch();
        } else if (event.key === 'Enter') {
          event.preventDefault();
          query = searchInput.value;
          closeSearch();
          findMatch(event.shiftKey);
        }
      });
    }
    const selection = getSelection();
    savedSelection = selection.rangeCount ? selection.getRangeAt(0).cloneRange() : null;
    searchBar.hidden = false;
    searchInput.value = query;
    searchStatus.textContent = 'Enter to find · Esc to close';
    searchInput.focus({ preventScroll: true });
    searchInput.select();
  }

  document.addEventListener('focusin', resetSequence);
  document.addEventListener('pointerdown', resetSequence);
  window.addEventListener('blur', resetSequence);

  document.addEventListener('keydown', event => {
    const target = event.target;
    if (event.defaultPrevented || event.isComposing || event.metaKey || event.altKey ||
        (target instanceof Element &&
          (target.closest('input, textarea, select') || target.isContentEditable))) {
      resetSequence();
      return;
    }

    const secondG = pendingG;
    const afterColon = pendingColon;
    resetSequence();
    const page = document.scrollingElement;
    if (!page) return;

    if (!event.ctrlKey && event.key === ':' && !event.repeat) {
      pendingColon = true;
    } else if (!event.ctrlKey && event.key === '/' && afterColon) {
      openSearch();
    } else if (!event.ctrlKey && (event.key === 'n' || event.key === 'N') && query) {
      findMatch(event.key === 'N');
    } else if (event.ctrlKey && !event.shiftKey && (event.key === 'u' || event.key === 'd')) {
      page.scrollTop += (event.key === 'd' ? 1 : -1) * document.documentElement.clientHeight / 2;
    } else if (!event.ctrlKey && event.key === 'G') {
      page.scrollTop = page.scrollHeight;
    } else if (!event.ctrlKey && !event.shiftKey && event.key === 'g') {
      // Holding g must not turn key repeat into a two-key command.
      if (!event.repeat) {
        if (secondG) page.scrollTop = 0;
        else pendingG = true;
      }
    } else {
      return;
    }
    event.preventDefault();
  });

  function scrollable(element) {
    return element instanceof Element &&
      element.matches('pre, .table-scroll') &&
      element.scrollWidth > element.clientWidth;
  }

  document.addEventListener('pointerdown', event => {
    if (!(event.target instanceof Element) || event.target.closest('a, input, button')) return;
    const region = event.target.closest('pre, .table-scroll');
    if (scrollable(region)) region.focus({ preventScroll: true });
  });

  document.addEventListener('keydown', event => {
    if (event.metaKey || event.ctrlKey || event.altKey || event.shiftKey) return;
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
    const region = document.activeElement;
    if (!scrollable(region)) return;
    const previous = region.scrollLeft;
    region.scrollLeft += event.key === 'ArrowRight' ? 48 : -48;
    if (region.scrollLeft !== previous) event.preventDefault();
  });

  document.addEventListener('wheel', event => {
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    const region = event.target instanceof Element
      ? event.target.closest('pre, .table-scroll') : null;
    if (!scrollable(region)) return;
    let delta = event.deltaX;
    if (event.shiftKey && !delta) delta = event.deltaY;
    else if (Math.abs(delta) < Math.abs(event.deltaY)) return;
    if (!delta) return;
    if (event.deltaMode === 1) delta *= parseFloat(getComputedStyle(region).lineHeight) || 24;
    if (event.deltaMode === 2) delta *= region.clientWidth;
    const previous = region.scrollLeft;
    region.scrollLeft += delta;
    if (region.scrollLeft !== previous) event.preventDefault();
  }, { passive: false });
})();
