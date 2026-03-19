/** Release asset metadata, UA-based OS/arch detection, asset lookup. */

export const releaseUrl =
  "https://api.github.com/repos/saveenergy/openbyte/releases/latest";
export const releasePage =
  "https://github.com/saveenergy/openbyte/releases/latest";

export const platforms = {
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

export const archLabels = {
  "linux_amd64.tar.gz": { arch: "x86_64", short: "amd64" },
  "linux_arm64.tar.gz": { arch: "ARM64", short: "arm64" },
  "darwin_amd64.tar.gz": { arch: "Intel", short: "amd64" },
  "darwin_arm64.tar.gz": { arch: "Apple Silicon", short: "arm64" },
  "windows_amd64.zip": { arch: "x86_64", short: "amd64" },
};

export function isMac(platform, ua) {
  return /Mac/i.test(platform) || /Mac/i.test(ua);
}

export function isWindows(platform, ua) {
  return /Win/i.test(platform) || /Win/i.test(ua);
}

export function userAgentDataArch() {
  const arch = navigator.userAgentData?.architecture;
  return typeof arch === "string" ? arch : "";
}

export function hasAppleSiliconRenderer() {
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

export function detectMacArch(platform, ua) {
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

export function detectPlatform() {
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

export function formatBytes(bytes) {
  if (typeof bytes !== "number" || !Number.isFinite(bytes) || bytes <= 0)
    return "";
  const mb = bytes / (1024 * 1024);
  return mb.toFixed(1) + " MB";
}

export function findAsset(assets, suffix) {
  return assets.find(function (a) {
    return (
      a &&
      typeof a.name === "string" &&
      a.name.startsWith("openbyte_") &&
      a.name.endsWith(suffix)
    );
  });
}

export function suffixForDetected(detected) {
  if (detected.os === "windows") return "windows_amd64.zip";
  const osKey = detected.os === "macos" ? "darwin" : "linux";
  return (
    osKey +
    "_" +
    detected.arch +
    (detected.os === "windows" ? ".zip" : ".tar.gz")
  );
}

export function altSuffix(detected) {
  if (detected.os === "windows") return null;
  const osKey = detected.os === "macos" ? "darwin" : "linux";
  const other = detected.arch === "arm64" ? "amd64" : "arm64";
  return osKey + "_" + other + ".tar.gz";
}
