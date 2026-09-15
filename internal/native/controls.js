// App-owned controls execute only in an isolated WKContentWorld. Markdown
// cannot supply scripts or reach this world. These handlers add Vim document
// navigation and predictable horizontal overflow controls.
(() => {
  let pendingG = false;
  const resetSequence = () => { pendingG = false; };

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
    resetSequence();
    const page = document.scrollingElement;
    if (!page) return;

    if (event.ctrlKey && !event.shiftKey && (event.key === 'u' || event.key === 'd')) {
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
