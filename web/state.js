/** Shared state, DOM element refs, and config constants. */

let _apiBase = "/api/v1";

export function getApiBase() {
  return _apiBase;
}

export const state = {
  phase: "idle",
  isRunning: false,
  downloadResult: 0,
  uploadResult: 0,
  latencyResult: null,
  jitterResult: null,
  downloadLatency: 0,
  uploadLatency: 0,
  currentSpeed: 0,
  progress: 0,
  abortController: null,
  servers: [],
  selectedServer: null,
  settings: {
    duration: 30,
    streams: 4,
  },
  networkInfo: {
    ipv4: null,
    ipv6: null,
  },
  resultId: null,
  shareSavePromise: null,
  lastAriaProgressUpdateMs: 0,
  lastAriaProgressValue: -1,
  /** Internal diagnostics (peak/sustained/volatility, stop_reason) — not shown in default UI */
  diagnostics: null,
};

/** Populated by `initElements()` after the document is ready (module load can precede DOM). */
export const elements = {};

export function initElements() {
  elements.idleState = document.getElementById("idleState");
  elements.testingState = document.getElementById("testingState");
  elements.resultsState = document.getElementById("resultsState");
  elements.startBtn = document.getElementById("startBtn");
  elements.speedNumber = document.getElementById("speedNumber");
  elements.speedUnit = document.getElementById("speedUnit");
  elements.testType = document.getElementById("testType");
  elements.progressMeter = document.getElementById("progressMeter");
  elements.progressRing = document.getElementById("progressRing");
  elements.downloadResult = document.getElementById("downloadResult");
  elements.uploadResult = document.getElementById("uploadResult");
  elements.latencyResult = document.getElementById("latencyResult");
  elements.jitterResult = document.getElementById("jitterResult");
  elements.loadedLatencyResult = document.getElementById("loadedLatencyResult");
  elements.bufferbloatResult = document.getElementById("bufferbloatResult");
  elements.serverName = document.getElementById("serverName");
  elements.networkIPv4 = document.getElementById("networkIPv4");
  elements.networkIPv6 = document.getElementById("networkIPv6");
  elements.restartBtn = document.getElementById("restartBtn");
  elements.cancelBtn = document.getElementById("cancelBtn");
  elements.serverInfo = document.getElementById("serverInfo");
  elements.serverDot = document.querySelector(".server-dot");
  elements.serverText = document.querySelector(".server-text");
  elements.showSettings = document.getElementById("showSettings");
  elements.closeSettings = document.getElementById("closeSettings");
  elements.settingsModal = document.getElementById("settingsModal");
  elements.duration = document.getElementById("duration");
  elements.streams = document.getElementById("streams");
  elements.serverSelectGroup = document.getElementById("serverSelectGroup");
  elements.serverSelect = document.getElementById("serverSelect");
  elements.serverStatus = document.getElementById("serverStatus");
  elements.errorToast = document.getElementById("errorToast");
  elements.errorMessage = document.getElementById("errorMessage");
  elements.successToast = document.getElementById("successToast");
  elements.successMessage = document.getElementById("successMessage");
  elements.shareBtn = document.getElementById("shareBtn");
}

export const RING_CIRCUMFERENCE = 2 * Math.PI * 90;
export const RING_END_OFFSET = 2;

export const TEST_CONFIG = {
  HTTP_TIMEOUT_BUFFER_MS: 10000,
  HEALTH_CHECK_TIMEOUT_MS: 5000,
  RETRY_AFTER_DEFAULT_MS: 1000,
  RETRY_AFTER_MAX_MS: 120000,
  STREAM_DELAY_MS: 200,
  MAX_NETWORK_RETRIES: 2,
  NETWORK_RETRY_DELAY_MS: 250,
  UPLOAD_RANDOM_CHUNK_BYTES: 65536,
  MIN_MEASURE_SECONDS: 0.001,
  PROGRESS_TICK_MS: 100,
  SPEED_UPDATE_MIN_INTERVAL_MS: 200,
  EWMA_ALPHA: 0.3,
  LATENCY_SAMPLE_COUNT: 24,
  LATENCY_WARMUP_PINGS: 2,
  LOADED_LATENCY_POLL_MS: 500,
  WARMUP_WINDOW_MS: 500,
  WARMUP_STABILITY_THRESHOLD: 0.15,
  WARMUP_REQUIRED_WINDOWS: 3,
  WARMUP_MAX_GRACE_RATIO: 0.3,
  WARMUP_MAX_GRACE_MS: 5000,
  TOAST_ERROR_MS: 5000,
  TOAST_SUCCESS_MS: 2000,
  ARIA_PROGRESS_UPDATE_MS: 1000,
};

export function setApiBase(base) {
  _apiBase = base;
}

export const modal = { lastTrigger: null };
export const toast = { timer: null };
