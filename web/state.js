/** Shared state, DOM element refs, and config constants. */

export let apiBase = "/api/v1";

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
  ws: null,
  streamId: null,
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
};

export const elements = {
  idleState: document.getElementById("idleState"),
  testingState: document.getElementById("testingState"),
  resultsState: document.getElementById("resultsState"),
  startBtn: document.getElementById("startBtn"),
  speedNumber: document.getElementById("speedNumber"),
  speedUnit: document.getElementById("speedUnit"),
  testType: document.getElementById("testType"),
  progressMeter: document.getElementById("progressMeter"),
  progressRing: document.getElementById("progressRing"),
  downloadResult: document.getElementById("downloadResult"),
  uploadResult: document.getElementById("uploadResult"),
  latencyResult: document.getElementById("latencyResult"),
  jitterResult: document.getElementById("jitterResult"),
  loadedLatencyResult: document.getElementById("loadedLatencyResult"),
  bufferbloatResult: document.getElementById("bufferbloatResult"),
  serverName: document.getElementById("serverName"),
  networkIPv4: document.getElementById("networkIPv4"),
  networkIPv6: document.getElementById("networkIPv6"),
  restartBtn: document.getElementById("restartBtn"),
  cancelBtn: document.getElementById("cancelBtn"),
  serverInfo: document.getElementById("serverInfo"),
  serverDot: document.querySelector(".server-dot"),
  serverText: document.querySelector(".server-text"),
  showSettings: document.getElementById("showSettings"),
  closeSettings: document.getElementById("closeSettings"),
  settingsModal: document.getElementById("settingsModal"),
  duration: document.getElementById("duration"),
  streams: document.getElementById("streams"),
  serverSelectGroup: document.getElementById("serverSelectGroup"),
  serverSelect: document.getElementById("serverSelect"),
  serverStatus: document.getElementById("serverStatus"),
  errorToast: document.getElementById("errorToast"),
  errorMessage: document.getElementById("errorMessage"),
  shareBtn: document.getElementById("shareBtn"),
};

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
  apiBase = base;
}

export const modal = { lastTrigger: null };
export const toast = { timer: null };
