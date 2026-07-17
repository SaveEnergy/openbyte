/**
 * Load the optional brand logo only on branded deployments. The logo img is
 * hidden until /branding.css sets it visible, so requesting it eagerly would
 * 404 on every unbranded deployment.
 */

for (const img of document.querySelectorAll(".brand-logo[data-src]")) {
  if (getComputedStyle(img).display !== "none") {
    img.src = img.dataset.src;
  }
  img.removeAttribute("data-src");
}
