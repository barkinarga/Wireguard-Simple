// V2 is the main design. Keep the original available only by explicit URL.
if (new URLSearchParams(location.search).get('design') === 'v1') {
  document.documentElement.dataset.design = 'v1';
  document.querySelector('#design-v2')?.remove();
}
