// App-owned controls execute only in an isolated WKContentWorld. Markdown
// cannot supply scripts or reach this world. Native WebKit handles vertical
// scrolling; these handlers make horizontal overflow predictable with a
// keyboard, horizontal wheel, or Shift-wheel.
(() => {
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
