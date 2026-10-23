/**
 * Load the optional brand logo only on branded deployments. The logo img is
 * hidden until /branding.css sets it visible, so requesting it eagerly would
 * 404 on every unbranded deployment.
 */

import "./theme.js";

const root = document.documentElement;
const sources = [];

function darkLogoMedia() {
  if (root.dataset.theme === "dark") return "all";
  if (root.dataset.theme === "light") return "not all";
  return "(prefers-color-scheme: dark)";
}

for (const img of document.querySelectorAll(".brand-logo[data-src]")) {
  if (getComputedStyle(img).display !== "none") {
    const picture = document.createElement("picture");
    picture.className = "brand-picture";
    const source = document.createElement("source");
    source.media = darkLogoMedia();
    source.srcset = `${img.dataset.src}?theme=dark`;
    img.before(picture);
    picture.append(source, img);
    img.src = img.dataset.src;
    sources.push(source);
  }
  img.removeAttribute("data-src");
}

// Picture follows system changes natively; explicit choices override its media.
if (sources.length) {
  new MutationObserver(() => {
    for (const source of sources) source.media = darkLogoMedia();
  }).observe(root, { attributes: true, attributeFilter: ["data-theme"] });
}
