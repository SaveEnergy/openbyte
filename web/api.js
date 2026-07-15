/** API docs page: show the concrete base URL (the API is same-origin). */

const baseUrlEl = document.getElementById("apiBaseUrl");
if (baseUrlEl && globalThis.location?.origin) {
  baseUrlEl.textContent = globalThis.location.origin;
}
