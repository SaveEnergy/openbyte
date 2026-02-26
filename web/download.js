/** Download page: GitHub releases, platform detection, copy buttons. */

const releaseUrl =
  "https://api.github.com/repos/saveenergy/openbyte/releases/latest";
const releasePage = "https://github.com/saveenergy/openbyte/releases/latest";

const platforms = {
  linux: {
    suffixes: ["linux_amd64.tar.gz", "linux_arm64.tar.gz"],
    icon: "🐧",
    name: "Linux",
  },
  macos: {
    suffixes: ["darwin_amd64.tar.gz", "darwin_arm64.tar.gz"],
    icon: "🍎",
    name: "macOS",
  },
  windows: { suffixes: ["windows_amd64.zip"], icon: "🪟", name: "Windows" },
};

const archLabels = {
  "linux_amd64.tar.gz": { arch: "x86_64", short: "amd64" },
  "linux_arm64.tar.gz": { arch: "ARM64", short: "arm64" },
  "darwin_amd64.tar.gz": { arch: "Intel", short: "amd64" },
  "darwin_arm64.tar.gz": { arch: "Apple Silicon", short: "arm64" },
  "windows_amd64.zip": { arch: "x86_64", short: "amd64" },
};

function isMac(platform, ua) {
  return /Mac/i.test(platform) || /Mac/i.test(ua);
}

function isWindows(platform, ua) {
  return /Win/i.test(platform) || /Win/i.test(ua);
}

function userAgentDataArch() {
  const arch = navigator.userAgentData?.architecture;
  return typeof arch === "string" ? arch : "";
}

function hasAppleSiliconRenderer() {
  try {
    const canvas = document.createElement("canvas");
    const gl = canvas.getContext("webgl");
    if (!gl) return false;
    const dbg = gl.getExtension("WEBGL_debug_renderer_info");
    if (!dbg) return false;
    const renderer = gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL);
    return /Apple/.test(renderer) && !/Intel/.test(renderer);
  } catch (e) {
    console.debug("download page: GPU renderer probe failed", e);
    return false;
  }
}

function detectMacArch(platform, ua) {
  const uaDataArch = userAgentDataArch();
  if (/arm|aarch64/i.test(uaDataArch) || /arm64|aarch64/i.test(ua)) {
    return "arm64";
  }
  if (/x86|amd64|x64/i.test(uaDataArch) && !/arm/i.test(uaDataArch)) {
    return "amd64";
  }
  if (!isMac(platform, ua)) return "amd64";
  // On Safari and privacy-reduced UAs, Intel token can still appear on Apple Silicon.
  // Probe renderer on macOS to improve package auto-selection accuracy.
  return hasAppleSiliconRenderer() ? "arm64" : "amd64";
}

function detectPlatform() {
  const ua = navigator.userAgent || "";
  // Prefer UA-CH platform; fallback relies on UA sniffing below.
  const platform = navigator.userAgentData?.platform || "";
  if (isMac(platform, ua)) {
    return { os: "macos", arch: detectMacArch(platform, ua) };
  }
  if (isWindows(platform, ua)) {
    return { os: "windows", arch: "amd64" };
  }
  if (/aarch64|arm64/i.test(ua)) {
    return { os: "linux", arch: "arm64" };
  }
  return { os: "linux", arch: "amd64" };
}

function formatBytes(bytes) {
  if (typeof bytes !== "number" || !Number.isFinite(bytes) || bytes <= 0)
    return "";
  const mb = bytes / (1024 * 1024);
  return mb.toFixed(1) + " MB";
}

function findAsset(assets, suffix) {
  return assets.find(function (a) {
    return (
      a &&
      typeof a.name === "string" &&
      a.name.startsWith("openbyte_") &&
      a.name.endsWith(suffix)
    );
  });
}

function suffixForDetected(detected) {
  if (detected.os === "windows") return "windows_amd64.zip";
  const osKey = detected.os === "macos" ? "darwin" : "linux";
  return (
    osKey +
    "_" +
    detected.arch +
    (detected.os === "windows" ? ".zip" : ".tar.gz")
  );
}

function altSuffix(detected) {
  if (detected.os === "windows") return null;
  const osKey = detected.os === "macos" ? "darwin" : "linux";
  const other = detected.arch === "arm64" ? "amd64" : "arm64";
  return osKey + "_" + other + ".tar.gz";
}

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

async function fetchLatestRelease() {
  const res = await fetch(releaseUrl);
  if (!res.ok) {
    const reason =
      res.status === 403
        ? "GitHub API rate limited"
        : "GitHub API error " + res.status;
    try {
      await res.text();
    } catch (err) {
      console.debug("download page: failed to read release error body", err);
    }
    throw new Error(reason);
  }
  return res.json();
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
