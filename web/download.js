/** Download page: GitHub releases, platform detection, copy buttons. */

import {
  platforms,
  archLabels,
  releasePage,
  detectPlatform,
  formatBytes,
  findAsset,
  suffixForDetected,
  altSuffix,
} from "./download-platform.js";
import { fetchLatestRelease } from "./download-github.js";

function setRecommendedButtonState(btn, enabled) {
  if (!btn) return;
  btn.classList.toggle("is-disabled", !enabled);
  btn.setAttribute("aria-disabled", enabled ? "false" : "true");
  if (enabled) {
    btn.removeAttribute("tabindex");
    return;
  }
  btn.setAttribute("tabindex", "-1");
}

function renderRecommended(assets, detected, version) {
  const suffix = suffixForDetected(detected);
  const asset = findAsset(assets, suffix);
  const info = archLabels[suffix] || {
    arch: detected.arch,
    short: detected.arch,
  };
  const platName = platforms[detected.os]
    ? platforms[detected.os].name
    : detected.os;

  const platformEl = document.getElementById("recommendedPlatform");
  const btn = document.getElementById("recommendedBtn");
  const label = document.getElementById("recommendedLabel");
  const meta = document.getElementById("recommendedMeta");
  if (!platformEl || !btn || !label || !meta) return;

  platformEl.textContent = platName + " · " + info.arch;

  if (asset?.browser_download_url) {
    btn.href = asset.browser_download_url;
    setRecommendedButtonState(btn, true);
    btn.rel = "noopener noreferrer";
    label.textContent = "Download " + (version || "");
    meta.textContent = info.arch + " · " + formatBytes(asset.size);
  } else {
    btn.href = releasePage;
    setRecommendedButtonState(btn, true);
    btn.rel = "noopener noreferrer";
    label.textContent = "View on GitHub";
    meta.textContent = "";
  }

  // Alt architecture link
  const alt = altSuffix(detected);
  const altEl = document.getElementById("altArch");
  if (alt && altEl) {
    const altAsset = findAsset(assets, alt);
    const altInfo = archLabels[alt];
    if (altAsset && altInfo && altAsset.browser_download_url) {
      const a = document.createElement("a");
      a.className = "dl-alt-link";
      a.href = altAsset.browser_download_url;
      a.rel = "noopener noreferrer";
      a.textContent =
        "Also available for " +
        altInfo.arch +
        " (" +
        formatBytes(altAsset.size) +
        ")";
      altEl.appendChild(a);
    }
  }
}

function renderAllPlatforms(assets) {
  Object.keys(platforms).forEach(function (osKey) {
    const container = document.querySelector('[data-os="' + osKey + '"]');
    if (!container) return;
    while (container.firstChild) container.firstChild.remove();
    const plat = platforms[osKey];
    let found = false;

    plat.suffixes.forEach(function (suffix) {
      const asset = findAsset(assets, suffix);
      if (!asset?.browser_download_url) return;
      found = true;
      const info = archLabels[suffix] || { arch: suffix, short: suffix };

      const row = document.createElement("a");
      row.className = "dl-asset-row";
      row.href = asset.browser_download_url;
      row.rel = "noopener noreferrer";

      const nameSpan = document.createElement("span");
      nameSpan.className = "dl-asset-name";
      nameSpan.textContent = info.arch;

      const sizeSpan = document.createElement("span");
      sizeSpan.className = "dl-asset-size";
      sizeSpan.textContent = formatBytes(asset.size);

      row.appendChild(nameSpan);
      row.appendChild(sizeSpan);
      container.appendChild(row);
    });

    if (!found) {
      const fallback = document.createElement("a");
      fallback.className = "download-link";
      fallback.href = releasePage;
      fallback.target = "_blank";
      fallback.rel = "noopener noreferrer";
      fallback.textContent = "View release";
      container.appendChild(fallback);
    }
  });
}

function renderInstall(detected, version) {
  const section = document.getElementById("installSection");
  const tabs = document.getElementById("installTabs");
  const cmd = document.getElementById("installCmd");
  if (!section || !tabs || !cmd) return;

  let commands = {};
  commands["curl"] =
    "curl -fsSL https://github.com/saveenergy/openbyte/releases/" +
    (version ? "download/" + version : "latest/download") +
    "/openbyte_" +
    (detected.os === "macos" ? "darwin" : "linux") +
    "_" +
    detected.arch +
    ".tar.gz | tar xz";

  commands["docker"] =
    "docker run -p 8080:8080 ghcr.io/saveenergy/openbyte:" +
    (version ? version.replace(/^v/, "") : "latest") +
    " server";

  if (detected.os === "windows") {
    commands = {};
    commands["powershell"] =
      'Invoke-WebRequest -Uri "https://github.com/saveenergy/openbyte/releases/' +
      (version ? "download/" + version : "latest/download") +
      '/openbyte_windows_amd64.zip" -OutFile openbyte.zip; Expand-Archive openbyte.zip';
    commands["docker"] =
      "docker run -p 8080:8080 ghcr.io/saveenergy/openbyte:" +
      (version ? version.replace(/^v/, "") : "latest") +
      " server";
  }

  const keys = Object.keys(commands);
  if (keys.length === 0) return;
  section.classList.remove("hidden");

  let activeTab = keys[0];

  function renderTabs() {
    while (tabs.firstChild) tabs.firstChild.remove();
    keys.forEach(function (key) {
      const btn = document.createElement("button");
      btn.className = "dl-install-tab" + (key === activeTab ? " active" : "");
      btn.textContent = key;
      btn.onclick = function () {
        activeTab = key;
        renderTabs();
      };
      tabs.appendChild(btn);
    });
    cmd.textContent = commands[activeTab];
  }
  renderTabs();
}

function renderVersion(data) {
  const tag = data.tag_name || "";
  const date = data.published_at ? new Date(data.published_at) : null;
  const el = document.getElementById("versionTag");
  const parts = [];
  if (tag) parts.push(tag);
  if (date && Number.isFinite(date.getTime())) {
    parts.push(
      date.toLocaleDateString(undefined, {
        year: "numeric",
        month: "short",
        day: "numeric",
      }),
    );
  }
  if (parts.length && el) el.textContent = " · " + parts.join(" · ");
}

function applyGithubFallback(err) {
  const btn = document.getElementById("recommendedBtn");
  const label = document.getElementById("recommendedLabel");
  const platform = document.getElementById("recommendedPlatform");
  if (btn) {
    btn.href = releasePage;
    setRecommendedButtonState(btn, true);
    btn.rel = "noopener noreferrer";
  }
  if (label) label.textContent = "View on GitHub";
  if (platform) {
    platform.textContent = err?.message || "Could not load release data";
  }
  document.querySelectorAll(".download-links").forEach(function (c) {
    while (c.firstChild) c.firstChild.remove();
    const link = document.createElement("a");
    link.className = "download-link";
    link.href = releasePage;
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    link.textContent = "View release";
    c.appendChild(link);
  });
}

function fallbackCopy(text) {
  const ta = document.createElement("textarea");
  ta.value = text;
  ta.setAttribute("readonly", "");
  ta.style.position = "absolute";
  ta.style.left = "-9999px";
  document.body.appendChild(ta);
  ta.select();
  ta.remove();
  // deprecated fallback intentionally dropped; clipboard API handles supported browsers
  return false;
}

function tryFallbackCopy(text, onSuccess, onFailure) {
  if (fallbackCopy(text)) {
    onSuccess();
    return true;
  }
  onFailure();
  return false;
}

function copyText(text, onSuccess, onFailure) {
  if (navigator.clipboard?.writeText) {
    navigator.clipboard
      .writeText(text)
      .then(onSuccess)
      .catch(function () {
        tryFallbackCopy(text, onSuccess, onFailure);
      });
    return;
  }
  tryFallbackCopy(text, onSuccess, onFailure);
}

function setTemporaryButtonText(btn, text) {
  btn.textContent = text;
  setTimeout(function () {
    btn.textContent = "Copy";
  }, 1500);
}

// Setup copy buttons
function setupCopy(btnId, getTextFn) {
  const btn = document.getElementById(btnId);
  if (!btn) return;
  btn.addEventListener("click", function () {
    const text = getTextFn();
    copyText(
      text,
      function () {
        setTemporaryButtonText(btn, "Copied!");
      },
      function () {
        setTemporaryButtonText(btn, "Failed");
      },
    );
  });
}

setupCopy("copyBtn", function () {
  const installCmd = document.getElementById("installCmd");
  return installCmd ? installCmd.textContent : "";
});
setupCopy("copyDockerBtn", function () {
  const btn = document.querySelector("#copyDockerBtn");
  const block = btn ? btn.closest(".dl-code-block") : null;
  const code = block ? block.querySelector("code") : null;
  return code ? code.textContent : "";
});

const detected = detectPlatform();

try {
  const data = await fetchLatestRelease();
  const assets = Array.isArray(data.assets) ? data.assets : [];
  renderVersion(data);
  renderRecommended(assets, detected, data.tag_name);
  renderAllPlatforms(assets);
  renderInstall(detected, data.tag_name);
} catch (err) {
  console.warn("Release fetch failed:", err);
  applyGithubFallback(err);
}
