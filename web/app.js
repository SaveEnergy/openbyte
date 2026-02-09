let apiBase = '/api/v1';

const state = {
  phase: 'idle',
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
    streams: 4,  // Default 4 streams for ~600 Mbps capacity
    serverUrl: ''
  },
  networkInfo: {
    ipv4: null,
    ipv6: null
  },
  resultId: null
};

const elements = {
  idleState: document.getElementById('idleState'),
  testingState: document.getElementById('testingState'),
  resultsState: document.getElementById('resultsState'),
  startBtn: document.getElementById('startBtn'),
  speedNumber: document.getElementById('speedNumber'),
  speedUnit: document.getElementById('speedUnit'),
  testType: document.getElementById('testType'),
  progressRing: document.getElementById('progressRing'),
  downloadResult: document.getElementById('downloadResult'),
  uploadResult: document.getElementById('uploadResult'),
  latencyResult: document.getElementById('latencyResult'),
  jitterResult: document.getElementById('jitterResult'),
  loadedLatencyResult: document.getElementById('loadedLatencyResult'),
  bufferbloatResult: document.getElementById('bufferbloatResult'),
  serverName: document.getElementById('serverName'),
  networkIPv4: document.getElementById('networkIPv4'),
  networkIPv6: document.getElementById('networkIPv6'),
  restartBtn: document.getElementById('restartBtn'),
  cancelBtn: document.getElementById('cancelBtn'),
  serverInfo: document.getElementById('serverInfo'),
  serverDot: document.querySelector('.server-dot'),
  serverText: document.querySelector('.server-text'),
  showSettings: document.getElementById('showSettings'),
  closeSettings: document.getElementById('closeSettings'),
  settingsModal: document.getElementById('settingsModal'),
  duration: document.getElementById('duration'),
  streams: document.getElementById('streams'),
  serverSelect: document.getElementById('serverSelect'),
  customServerUrl: document.getElementById('customServerUrl'),
  serverStatus: document.getElementById('serverStatus'),
  errorToast: document.getElementById('errorToast'),
  errorMessage: document.getElementById('errorMessage'),
  shareBtn: document.getElementById('shareBtn')
};

const RING_CIRCUMFERENCE = 2 * Math.PI * 90;
const RING_END_OFFSET = 2;
let lastModalTrigger = null;
let toastTimer = null;

const sleep = (ms) => new Promise(r => setTimeout(r, ms));

const isNetworkError = (err) => {
  if (!err) return false;
  if (err.name === 'AbortError') return false;
  if (err.name === 'TypeError') return true;
  const message = (err.message || '').toLowerCase();
  return message.includes('network') || message.includes('failed to fetch') || message.includes('http2');
};

document.addEventListener('DOMContentLoaded', () => {
  loadSettings();
  loadServers();
  bindEvents();
});

function bindEvents() {
  if (!elements.startBtn || !elements.restartBtn || !elements.duration || !elements.streams) {
    console.warn('Core UI elements missing; skipping event binding');
    return;
  }
  elements.startBtn.addEventListener('click', startTest);
  elements.restartBtn.addEventListener('click', resetToIdle);
  if (elements.cancelBtn) {
    elements.cancelBtn.addEventListener('click', resetToIdle);
  }
  if (elements.shareBtn) {
    elements.shareBtn.addEventListener('click', handleShare);
  }
  initSettingsModal();
  elements.duration.addEventListener('change', saveSettings);
  elements.streams.addEventListener('change', saveSettings);
  
  if (elements.serverSelect) {
    elements.serverSelect.addEventListener('change', onServerChange);
  }
  if (elements.customServerUrl) {
    elements.customServerUrl.addEventListener('change', onCustomServerChange);
    elements.customServerUrl.addEventListener('blur', onCustomServerChange);
  }
  
  detectNetworkInfo();
}

function initSettingsModal() {
  if (!elements.settingsModal || !elements.showSettings || !elements.closeSettings) return;

  const focusFirstSetting = () => {
    if (elements.duration) elements.duration.focus();
  };

  const openModal = () => {
    lastModalTrigger = document.activeElement;
    elements.settingsModal.showModal();
    requestAnimationFrame(focusFirstSetting);
  };

  const closeModal = () => {
    elements.settingsModal.close();
    if (lastModalTrigger && typeof lastModalTrigger.focus === 'function') {
      lastModalTrigger.focus();
    }
  };

  elements.showSettings.addEventListener('click', openModal);
  elements.closeSettings.addEventListener('click', closeModal);
  elements.settingsModal.addEventListener('cancel', (e) => {
    e.preventDefault();
    closeModal();
  });
  elements.settingsModal.addEventListener('click', (e) => {
    if (e.target === elements.settingsModal) closeModal();
  });
}

function loadSettings() {
  const saved = localStorage.getItem('obyte-settings');
  if (saved) {
    try {
      const s = JSON.parse(saved);
      // Validate parsed values before applying
      if (typeof s.duration === 'number' && Number.isFinite(s.duration) && s.duration > 0) {
        state.settings.duration = s.duration;
      }
      if (typeof s.streams === 'number' && Number.isFinite(s.streams) && s.streams > 0) {
        state.settings.streams = s.streams;
      }
      if (typeof s.serverUrl === 'string') {
        state.settings.serverUrl = s.serverUrl;
      }
      if (elements.duration) elements.duration.value = state.settings.duration;
      if (elements.streams) elements.streams.value = state.settings.streams;
      if (state.settings.serverUrl && elements.customServerUrl) {
        elements.customServerUrl.value = state.settings.serverUrl;
      }
    } catch (e) {
      console.warn('Failed to parse saved settings:', e);
    }
  }
}

function saveSettings() {
  if (!elements.duration || !elements.streams) return;
  const d = parseInt(elements.duration.value, 10);
  const s = parseInt(elements.streams.value, 10);
  if (Number.isFinite(d) && d > 0) state.settings.duration = d;
  if (Number.isFinite(s) && s > 0) state.settings.streams = s;
  localStorage.setItem('obyte-settings', JSON.stringify(state.settings));
  notifySettingsSaved();
}

function detectNetworkInfo() {
  // Main ping — captures whichever address family the browser chose
  const mainPing = fetch(`${apiBase}/ping`)
    .then(res => res.ok ? res.json() : res.text().then(() => Promise.reject()))
    .then(data => {
      if (data.client_ip) {
        if (data.ipv6) {
          state.networkInfo.ipv6 = data.client_ip;
        } else {
          state.networkInfo.ipv4 = data.client_ip;
        }
      }
    })
    .catch(() => {});

  // Dedicated probes — A-only / AAAA-only subdomains force address family
  const hostname = window.location.hostname;
  const canProbe = hostname && hostname !== 'localhost' &&
    !hostname.startsWith('v4.') && !hostname.startsWith('v6.') &&
    !hostname.startsWith('[') &&
    !hostname.match(/^\d/);

  const probeOpts = { cache: 'no-store', credentials: 'omit', mode: 'cors' };
  const proto = window.location.protocol;

  const v4Ping = canProbe
    ? fetch(`${proto}//v4.${hostname}/api/v1/ping`, probeOpts)
        .then(res => res.ok ? res.json() : res.text().then(() => Promise.reject()))
        .then(data => {
          if (!data.ipv6 && data.client_ip) {
            state.networkInfo.ipv4 = data.client_ip;
          }
        })
        .catch(() => {})
    : Promise.resolve();

  const v6Ping = canProbe
    ? fetch(`${proto}//v6.${hostname}/api/v1/ping`, probeOpts)
        .then(res => res.ok ? res.json() : res.text().then(() => Promise.reject()))
        .then(data => {
          if (data.ipv6 && data.client_ip) {
            state.networkInfo.ipv6 = data.client_ip;
          }
        })
        .catch(() => {})
    : Promise.resolve();

  Promise.allSettled([mainPing, v4Ping, v6Ping]).then(() => updateNetworkDisplay());
}

function updateNetworkDisplay() {
  if (elements.networkIPv4) {
    elements.networkIPv4.textContent = state.networkInfo.ipv4 || '-';
  }
  if (elements.networkIPv6) {
    elements.networkIPv6.textContent = state.networkInfo.ipv6 || '-';
  }
}

function resolveStreams() {
  if (!state.settings.streams || Number.isNaN(state.settings.streams)) {
    return 4;
  }
  return state.settings.streams;
}

function resolveChunkSize() {
  return 1024 * 1024;
}

// Detect HTTP protocol and return appropriate overhead factor
function detectOverheadFactor() {
  try {
    const entries = performance.getEntriesByType('resource');
    // Find the most recent API fetch to detect protocol
    for (let i = entries.length - 1; i >= 0; i--) {
      const e = entries[i];
      if (e.name && e.name.includes('/api/v1/') && e.nextHopProtocol) {
        if (e.nextHopProtocol === 'h2' || e.nextHopProtocol === 'h3') {
          // HTTP/2: ~9 bytes frame header per chunk — negligible
          return 1.0;
        }
        // HTTP/1.1: chunked encoding + headers overhead
        return 1.02;
      }
    }
  } catch (_) {}
  // Fallback: conservative estimate
  return 1.02;
}

function isSameOriginURL(url) {
  try {
    const parsed = new URL(url, window.location.origin);
    return parsed.origin === window.location.origin;
  } catch (e) {
    return false;
  }
}

function fetchWithTimeout(url, options, timeoutMs) {
  if (typeof AbortController === 'undefined' || !timeoutMs) {
    return fetch(url, options);
  }
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);

  // Chain: if caller supplied an abort signal, propagate it
  const externalSignal = options?.signal;
  let onAbort = null;
  if (externalSignal) {
    if (externalSignal.aborted) {
      controller.abort();
    } else {
      onAbort = () => controller.abort();
      externalSignal.addEventListener('abort', onAbort, { once: true });
    }
  }

  const opts = { ...options, signal: controller.signal };
  return fetch(url, opts).finally(() => {
    clearTimeout(timer);
    if (onAbort && externalSignal) {
      externalSignal.removeEventListener('abort', onAbort);
    }
  });
}

function resolveServerName() {
  if (state.selectedServer?.name) {
    return state.selectedServer.name;
  }
  if (state.servers?.length) {
    const fallback = state.servers[0]?.name;
    if (fallback) return fallback;
  }
  return 'Current Server';
}

function updateServerName() {
  if (elements.serverName) {
    elements.serverName.textContent = resolveServerName();
  }
}

function getHealthURL(server) {
  if (server.api_endpoint) {
    try {
      const apiURL = new URL(server.api_endpoint);
      if (window.location.protocol === 'https:' && apiURL.protocol === 'http:') {
        apiURL.protocol = 'https:';
      }
      apiURL.pathname = apiURL.pathname.replace(/\/+$/, '') + '/health';
      return apiURL.toString();
    } catch (e) {
      return `${server.api_endpoint}/health`;
    }
  }
  const protocol = window.location.protocol || 'http:';
  return `${protocol}//${server.host}/health`;
}

async function loadServers() {
  try {
    const res = await fetch(`${apiBase}/servers`);
    if (!res.ok) {
      await res.text().catch(() => {});
      throw new Error(`Failed to load servers: HTTP ${res.status}`);
    }
    let data;
    try {
      data = await res.json();
    } catch (err) {
      state.servers = [];
      throw new Error('Failed to parse servers response');
    }
    state.servers = Array.isArray(data.servers) ? data.servers : [];
    
    if (state.servers.length > 0) {
      await selectFastestServer();
    } else {
      state.selectedServer = null;
    }
    
    populateServerSelect();
    updateServerName();
    checkServer();
  } catch (e) {
    console.error('Failed to load servers:', e);
    updateServerName();
    checkServer();
  }
}

async function selectFastestServer() {
  if (state.servers.length === 0) {
    checkServer();
    return;
  }

  if (elements.serverText) elements.serverText.textContent = 'Finding fastest...';

  const latencyPromises = state.servers.map(async (server) => {
    const healthUrl = getHealthURL(server);
    const isSameOrigin = isSameOriginURL(healthUrl);
    
    const start = performance.now();
    try {
      const res = await fetchWithTimeout(healthUrl, { 
        method: 'GET',
        mode: isSameOrigin ? 'same-origin' : 'cors'
      }, 5000);
      const latency = performance.now() - start;
      
      if (res.ok) {
        // Consume body to free connection for reuse
        await res.text().catch(() => {});
        return { server, latency, error: null };
      }
      await res.text().catch(() => {});
      return { server, latency: Infinity, error: 'unhealthy' };
    } catch (e) {
      return { server, latency: Infinity, error: e.message };
    }
  });

  const results = await Promise.all(latencyPromises);
  
  results.forEach(({ server, latency, error }) => {
    server.reachable = error === null;
    server.latency = error === null ? latency : null;
  });
  
  const reachable = results.filter(r => r.error === null);
  
  if (reachable.length === 0) {
    console.warn('No servers reachable, defaulting to current');
    state.selectedServer = null;
    updateServerName();
    return;
  }
  
  reachable.sort((a, b) => a.latency - b.latency);
  state.selectedServer = reachable[0].server;
  updateServerName();
  
  console.log('Auto-selected server:', state.selectedServer.name, 
    `(${Math.round(reachable[0].latency)}ms)`);
}

function populateServerSelect() {
  if (!elements.serverSelect) return;
  
  while (elements.serverSelect.firstChild) elements.serverSelect.removeChild(elements.serverSelect.firstChild);
  
  const currentOpt = document.createElement('option');
  currentOpt.value = 'current';
  currentOpt.textContent = 'Current Server';
  elements.serverSelect.appendChild(currentOpt);
  
  const reachableServers = state.servers.filter(server => server.api_endpoint && server.reachable);
  reachableServers.forEach(server => {
    const opt = document.createElement('option');
    opt.value = server.id;
    const location = server.location ? ` (${server.location})` : '';
    opt.textContent = `${server.name}${location}`;
    elements.serverSelect.appendChild(opt);
  });
  
  const customOpt = document.createElement('option');
  customOpt.value = 'custom';
  customOpt.textContent = 'Custom Server...';
  elements.serverSelect.appendChild(customOpt);
  
  if (state.settings.serverUrl) {
    elements.serverSelect.value = 'custom';
    showCustomServerInput(true);
  } else if (state.selectedServer?.id && reachableServers.some(s => s.id === state.selectedServer.id)) {
    elements.serverSelect.value = state.selectedServer.id;
    showCustomServerInput(false);
  } else {
    elements.serverSelect.value = 'current';
    showCustomServerInput(false);
  }
}

function onServerChange() {
  const value = elements.serverSelect.value;
  
  if (value === 'custom') {
    showCustomServerInput(true);
    return;
  }
  
  showCustomServerInput(false);
  
  if (value === 'current') {
    apiBase = '/api/v1';
    state.settings.serverUrl = '';
    state.selectedServer = null;
  } else {
    const server = state.servers.find(s => s.id === value);
    if (server) {
      apiBase = server.api_endpoint ? `${server.api_endpoint}/api/v1` : '/api/v1';
      state.settings.serverUrl = server.api_endpoint || '';
      state.selectedServer = server;
    }
  }
  
  updateServerName();
  saveSettings();
  checkServer();
}

function onCustomServerChange() {
  const url = elements.customServerUrl.value.trim();
  if (!url) {
    apiBase = '/api/v1';
    state.settings.serverUrl = '';
    state.selectedServer = null;
    updateServerName();
    saveSettings();
    checkServer();
    return;
  }
  let serverUrl = url;
  if (!/^https?:\/\//i.test(serverUrl)) {
    const preferred = window.location.protocol === 'https:' ? 'https://' : 'http://';
    serverUrl = preferred + serverUrl;
  }
  serverUrl = serverUrl.replace(/\/+$/, '');

  const apiSuffix = '/api/v1';
  let baseUrl = serverUrl;
  if (baseUrl.toLowerCase().endsWith(apiSuffix)) {
    baseUrl = baseUrl.slice(0, -apiSuffix.length);
  }
  baseUrl = baseUrl.replace(/\/+$/, '');
  
  apiBase = `${baseUrl}/api/v1`;
  state.settings.serverUrl = baseUrl;
  state.selectedServer = { id: 'custom', name: 'Custom', location: url, api_endpoint: baseUrl };
  updateServerName();
  saveSettings();
  checkServer();
}

function showCustomServerInput(show) {
  const container = elements.customServerUrl?.parentElement;
  if (container) {
    container.style.display = show ? 'block' : 'none';
  }
}

async function checkServer() {
  const baseUrl = state.settings.serverUrl;
  const candidates = baseUrl
    ? [`${baseUrl}/health`, `${apiBase}/health`, `${apiBase}/ping`]
    : ['/health', `${apiBase}/health`, `${apiBase}/ping`];
  
  try {
    let ok = false;
    for (const url of candidates) {
      try {
        const res = await fetchWithTimeout(url, {}, 5000);
        if (!res.ok) {
          await res.text().catch(() => {});
          continue;
        }
        
        const data = await res.json();
        if (data.status === 'ok' || data.status === 'healthy' || data.pong === true) {
          ok = true;
          break;
        }
      } catch (e) {
        continue;
      }
    }
    
    if (!ok) {
      throw new Error('Server offline');
    }
    
    if (elements.serverDot) {
      elements.serverDot.classList.remove('error', 'warning');
      elements.serverDot.classList.add('connected');
    }
    
    if (state.selectedServer) {
      if (elements.serverText) elements.serverText.textContent = state.selectedServer.name || 'Ready';
    } else {
      if (elements.serverText) elements.serverText.textContent = 'Ready';
    }
    
    if (elements.serverStatus) {
      elements.serverStatus.textContent = 'Connected';
      elements.serverStatus.className = 'server-status connected';
    }
  } catch (e) {
    if (baseUrl) {
      if (elements.serverDot) {
        elements.serverDot.classList.remove('connected', 'error');
        elements.serverDot.classList.add('warning');
      }
      if (elements.serverText) elements.serverText.textContent = state.selectedServer?.name || 'Custom';
      
      if (elements.serverStatus) {
        elements.serverStatus.textContent = 'Unverified';
        elements.serverStatus.className = 'server-status warning';
      }
    } else {
      if (elements.serverDot) {
        elements.serverDot.classList.remove('connected', 'warning');
        elements.serverDot.classList.add('error');
      }
      if (elements.serverText) elements.serverText.textContent = 'Offline';
      
      if (elements.serverStatus) {
        elements.serverStatus.textContent = 'Offline';
        elements.serverStatus.className = 'server-status error';
      }
    }
  }
}

async function startTest() {
  if (state.isRunning) {
    showError('Test already in progress');
    return;
  }
  
  state.isRunning = true;
  state.abortController = new AbortController();
  const signal = state.abortController.signal;
  
  try {
    state.phase = 'latency';
    showState('testing');
    updateTestType('◎ Latency', 'measuring');
    
    const latency = await measureLatency(signal);
    state.latencyResult = latency;
    
    if (signal.aborted) return;
    
    state.phase = 'download';
    resetProgress();
    updateTestType('↓ Download', 'downloading');
    
    // Yield to render UI before starting test
    await new Promise(r => requestAnimationFrame(r));
    
    const downloadSpeed = await runTest('download', signal);
    state.downloadResult = downloadSpeed;
    
    if (signal.aborted) return;
    
    state.phase = 'upload';
    resetProgress();
    updateTestType('↑ Upload', 'uploading');
    
    // Yield to render UI before starting test
    await new Promise(r => requestAnimationFrame(r));
    
    const uploadSpeed = await runTest('upload', signal);
    state.uploadResult = uploadSpeed;
    
    state.phase = 'results';
    showResults();
    
  } catch (e) {
    console.error('Test failed:', e);
    if (e.name !== 'AbortError') {
      const isCustom = !!state.settings.serverUrl;
      const message = isCustom && e instanceof TypeError
        ? 'Custom server unreachable or blocked by CORS.'
        : (e.message || 'Test failed');
      showError(message);
    }
    resetToIdle();
  } finally {
    // Only clean up if this is still the active test
    if (state.abortController?.signal === signal) {
      state.isRunning = false;
      state.abortController = null;
    }
  }
}

async function runTest(direction, signal) {
  if (signal.aborted) {
    throw new Error('Test cancelled');
  }
  
  const duration = state.settings.duration;
  const startTime = performance.now();
  let lastUpdate = startTime;
  let lastBytes = 0;
  let ewmaSpeed = 0;
  const ewmaAlpha = 0.3; // ~1s effective smoothing window
  
  // Smooth ring animation: advance based on wall clock time, independent of data callbacks
  const progressTick = setInterval(() => {
    if (signal.aborted) return;
    const elapsed = (performance.now() - startTime) / 1000;
    updateProgress(Math.min(100, (elapsed / duration) * 100));
  }, 100);

  const onProgress = (bytes, elapsed) => {
    if (elapsed > duration) return; // Past end time — freeze display
    const now = performance.now();
    const intervalMs = now - lastUpdate;
    
    if (intervalMs >= 200) {
      // Reset tracking on warmup-to-measurement transition (bytes counter resets)
      if (bytes < lastBytes) {
        lastBytes = bytes;
        ewmaSpeed = 0;
      }
      const intervalBytes = bytes - lastBytes;
      
      if (intervalBytes > 0 && intervalMs > 0) {
        const instantSpeed = (intervalBytes * 8) / (intervalMs / 1000) / 1_000_000;
        // EWMA smoothing for stable live display
        ewmaSpeed = ewmaSpeed === 0
          ? instantSpeed
          : ewmaAlpha * instantSpeed + (1 - ewmaAlpha) * ewmaSpeed;
        updateSpeed(Math.max(0, ewmaSpeed), direction);
      }
      
      updateProgress(Math.min(100, (elapsed / duration) * 100));
      
      lastUpdate = now;
      lastBytes = Math.max(lastBytes, bytes);
    }
  };
  
  // Run loaded latency probe in background during the test
  const latencyProbe = startLoadedLatencyProbe(signal);
  
  let result;
  try {
    if (direction === 'download') {
      result = await runDownloadTest(duration, onProgress, signal);
    } else {
      result = await runUploadTest(duration, onProgress, signal);
    }
  } finally {
    clearInterval(progressTick);
    await latencyProbe.stop();
  }
  const loadedLatency = latencyProbe.getMedian();
  if (direction === 'download') {
    state.downloadLatency = loadedLatency;
  } else {
    state.uploadLatency = loadedLatency;
  }

  if (state.isRunning) {
    updateProgress(100);
  }

  return result;
}

async function measureLatency(signal) {
  const rawSamples = [];
  const numSamples = 24;
  const warmUpPings = 2; // First pings include DNS/TLS overhead
  let capturedIP = false;
  
  for (let i = 0; i < numSamples; i++) {
    if (signal.aborted) break;
    
    const start = performance.now();
    try {
      const res = await fetch(`${apiBase}/ping`, { 
        method: 'GET',
        signal
      });
      const rtt = performance.now() - start;
      
      // Capture IP info from first successful response
      if (!capturedIP && res.ok) {
        try {
          const data = await res.json();
          if (data.client_ip) {
            if (data.ipv6) {
              state.networkInfo.ipv6 = data.client_ip;
            } else {
              state.networkInfo.ipv4 = data.client_ip;
            }
            updateNetworkDisplay();
          }
          capturedIP = true;
        } catch (_) {
          // JSON parse failed; skip
        }
      } else {
        await res.text().catch(() => {});
      }
      
      rawSamples.push(rtt);
      
      updateProgress((i / numSamples) * 100);
      updateSpeed(rtt, 'latency');
    } catch (e) {
      if (e.name === 'AbortError') break;
    }
  }
  
  if (rawSamples.length === 0) return null;
  
  // Discard warm-up pings (DNS/TLS overhead)
  const samples = rawSamples.length > warmUpPings
    ? rawSamples.slice(warmUpPings)
    : rawSamples;
  
  // IQR outlier filter
  const filtered = filterOutliersIQR(samples);
  
  if (filtered.length >= 2) {
    let sumDiff = 0;
    for (let i = 1; i < filtered.length; i++) {
      sumDiff += Math.abs(filtered[i] - filtered[i - 1]);
    }
    state.jitterResult = sumDiff / (filtered.length - 1);
  } else {
    state.jitterResult = 0;
  }
  
  filtered.sort((a, b) => a - b);
  const median = filtered[Math.floor(filtered.length / 2)];
  
  return median;
}

function computeBufferbloatGrade(idleLatency, loadedLatency) {
  if (!Number.isFinite(idleLatency) || !Number.isFinite(loadedLatency)) return null;
  if (idleLatency <= 0 || loadedLatency <= 0) return null;
  const increase = loadedLatency - idleLatency;
  if (increase < 5) return 'A+';
  if (increase < 15) return 'A';
  if (increase < 30) return 'B';
  if (increase < 60) return 'C';
  if (increase < 150) return 'D';
  return 'F';
}

function filterOutliersIQR(samples) {
  if (samples.length < 4) return samples.slice();
  const sorted = samples.slice().sort((a, b) => a - b);
  const q1 = sorted[Math.floor(sorted.length * 0.25)];
  const q3 = sorted[Math.floor(sorted.length * 0.75)];
  const iqr = q3 - q1;
  const lower = q1 - 1.5 * iqr;
  const upper = q3 + 1.5 * iqr;
  return samples.filter(s => s >= lower && s <= upper);
}

// Background latency measurement during load (bufferbloat detection)
function startLoadedLatencyProbe(signal) {
  const samples = [];
  let running = true;
  
  const loop = async () => {
    while (running && !signal.aborted) {
      const start = performance.now();
      try {
        const res = await fetch(`${apiBase}/ping`, {
          method: 'GET',
          cache: 'no-store',
          signal
        });
        samples.push(performance.now() - start);
        await res.text().catch(() => {});
      } catch (_) {
        if (!running) break;
      }
      await sleep(500);
    }
  };
  
  const promise = loop();
  
  return {
    stop() { running = false; return promise; },
    getMedian() {
      if (samples.length === 0) return 0;
      const filtered = filterOutliersIQR(samples);
      if (filtered.length === 0) return 0;
      filtered.sort((a, b) => a - b);
      return filtered[Math.floor(filtered.length / 2)];
    }
  };
}

// Dynamic warm-up: detects when throughput stabilizes
function createWarmUpDetector(durationMs) {
  const windowMs = 500;
  const stabilityThreshold = 0.15; // 15% variance
  const requiredStableWindows = 3;
  const maxGraceMs = Math.min(durationMs * 0.3, 5000);
  
  let windowBytes = 0;
  let windowStart = 0;
  let detectorStart = 0;
  let recentSpeeds = [];
  let settled = false;
  
  return {
    settled() { return settled; },
    
    record(bytes, now) {
      if (settled) return;
      if (detectorStart === 0) {
        detectorStart = now;
        windowStart = now;
      }
      windowBytes += bytes;
      const windowElapsed = now - windowStart;
      
      if (windowElapsed >= windowMs) {
        const speed = (windowBytes * 8) / (windowElapsed / 1000);
        recentSpeeds.push(speed);
        windowBytes = 0;
        windowStart = now;
        
        // Check stability over last N windows
        if (recentSpeeds.length >= requiredStableWindows) {
          const recent = recentSpeeds.slice(-requiredStableWindows);
          const avg = recent.reduce((a, b) => a + b) / recent.length;
          if (avg === 0) {
            // All windows zero — stalled but stable
            settled = true;
            return;
          }
          const maxDev = Math.max(...recent.map(s => Math.abs(s - avg) / avg));
          if (maxDev < stabilityThreshold) {
            settled = true;
            return;
          }
        }
        
        // Hard cap: don't spend more than maxGraceMs warming up
        if (now - detectorStart > maxGraceMs) {
          settled = true;
        }
      }
    }
  };
}

// HTTP-based download test
async function runDownloadTest(duration, onProgress, signal) {
  const startTime = performance.now();
  const numStreams = resolveStreams();
  const chunkSize = resolveChunkSize();
  const streamDelay = 200;
  const maxNetworkRetries = 2;
  const retryDelayMs = 250;
  const endTime = startTime + (duration * 1000);
  let totalBytes = 0;
  let allBytes = 0;
  let sawNetworkError = false;
  
  const warmUp = createWarmUpDetector(duration * 1000);
  let measureStartTime = 0;
  
  const streamPromises = [];
  
  const downloadStream = async (chunk) => {
    const res = await fetchWithTimeout(`${apiBase}/download?duration=${duration}&chunk=${chunk}`, {
      method: 'GET',
      cache: 'no-store',
      credentials: 'omit',
      signal: signal
    }, (duration * 1000) + 10000);
    
    if (!res.ok || !res.body) {
      await res.text().catch(() => {});
      if (res.status === 503) {
        const err = new Error('Server overloaded');
        err.status = 503;
        throw err;
      }
      return false;
    }
    
    const reader = res.body.getReader();
    try {
      while (true) {
        if (signal.aborted) break;

        const now = performance.now();
        if (now >= endTime) {
          await reader.cancel();
          break;
        }

        const { done, value } = await reader.read();
        if (done) break;
        
        allBytes += value.length;
        
        if (!warmUp.settled()) {
          warmUp.record(value.length, now);
          if (warmUp.settled()) {
            // Warm-up just ended — reset measurement
            totalBytes = 0;
            measureStartTime = now;
          }
        } else {
          totalBytes += value.length;
        }
        
        const elapsedSec = (now - startTime) / 1000;
        const displayBytes = warmUp.settled() ? totalBytes : allBytes;
        onProgress(displayBytes, elapsedSec);
      }
    } finally {
      reader.releaseLock();
    }
    return true;
  };
  
  const buildChunkAttempts = () => {
    const preferredFallback = 256 * 1024;
    const attempts = [chunkSize];
    if (preferredFallback < chunkSize) {
      attempts.push(preferredFallback);
    }
    if (65536 < attempts[attempts.length - 1]) {
      attempts.push(65536);
    }
    return attempts;
  };

  for (let i = 0; i < numStreams; i++) {
    const delay = i * streamDelay;
    
    const streamPromise = (async () => {
      await new Promise(r => setTimeout(r, delay));
      
      const attempts = buildChunkAttempts();
      for (let attemptIndex = 0; attemptIndex < attempts.length; attemptIndex++) {
        if (signal.aborted) return;
        const attemptChunk = attempts[attemptIndex];
        let success = false;
        for (let retry = 0; retry <= maxNetworkRetries; retry++) {
          if (signal.aborted) return;
          try {
            if (await downloadStream(attemptChunk)) {
              success = true;
              break;
            }
          } catch (e) {
            if (e.name === 'AbortError' || signal.aborted) {
              return;
            }
            if (e.status === 503) {
              await sleep(500);
              return;
            }
            if (isNetworkError(e)) {
              sawNetworkError = true;
              if (retry < maxNetworkRetries) {
                await sleep(retryDelayMs);
                continue;
              }
            }
            if (attemptIndex < attempts.length - 1) {
              console.warn('Download stream failed, retrying smaller chunk', e);
            } else {
              console.warn('Download stream failed after retries', e);
            }
            break;
          }
        }
        if (success) {
          return;
        }
      }
    })();
    
    streamPromises.push(streamPromise);
  }
  
  await Promise.all(streamPromises);
  
  const overheadFactor = detectOverheadFactor();
  const endNow = Math.min(performance.now(), endTime);
  const actualMeasureStart = measureStartTime > 0 ? measureStartTime : startTime;
  const measureTime = Math.max(0.001, (endNow - actualMeasureStart) / 1000);
  const avgSpeed = (totalBytes * 8 * overheadFactor) / measureTime / 1_000_000;
  
  if (totalBytes === 0 && sawNetworkError) {
    throw new Error('Network error during download. Try again or change server.');
  }

  return avgSpeed > 0 ? avgSpeed : 0;
}

async function runUploadTest(duration, onProgress, signal) {
  const startTime = performance.now();
  const numStreams = resolveStreams();
  const chunkSize = resolveChunkSize();
  const streamDelay = 200;
  const blobSize = chunkSize; // 1MB — balances HTTP overhead vs progress granularity
  const maxNetworkRetries = 2;
  const retryDelayMs = 250;
  let totalBytes = 0;
  let allBytes = 0;
  let sawNetworkError = false;
  
  const warmUp = createWarmUpDetector(duration * 1000);
  let measureStartTime = 0;
  
  const chunks = [];
  for (let i = 0; i < blobSize; i += 65536) {
    const piece = new Uint8Array(Math.min(65536, blobSize - i));
    crypto.getRandomValues(piece);
    chunks.push(piece);
  }
  const blob = new Blob(chunks);
  
  const endTime = startTime + (duration * 1000);
  
  const uploadStream = async (delay) => {
    await new Promise(r => setTimeout(r, delay));
    
    let consecutiveErrors = 0;
    while (performance.now() < endTime && !signal.aborted) {
      try {
        const res = await fetchWithTimeout(`${apiBase}/upload`, {
          method: 'POST',
          body: blob,
          headers: { 'Content-Type': 'application/octet-stream' },
          cache: 'no-store',
          credentials: 'omit',
          signal: signal
        }, (duration * 1000) + 10000);
        
        if (!res.ok) {
          await res.text().catch(() => {});
          if (res.status === 503) {
            await sleep(500);
            break;
          }
          consecutiveErrors += 1;
          if (consecutiveErrors > maxNetworkRetries) break;
          await sleep(retryDelayMs);
          continue;
        }
        consecutiveErrors = 0;
        await res.text().catch(() => {}); // drain body for HTTP/2 stream reuse
        
        const now = performance.now();
        allBytes += blobSize;
        
        if (!warmUp.settled()) {
          warmUp.record(blobSize, now);
          if (warmUp.settled()) {
            totalBytes = 0;
            measureStartTime = now;
          }
        } else {
          totalBytes += blobSize;
        }
        
        const elapsedSec = (now - startTime) / 1000;
        const displayBytes = warmUp.settled() ? totalBytes : allBytes;
        onProgress(displayBytes, elapsedSec);
        
      } catch (e) {
        if (e.name === 'AbortError') break;
        if (isNetworkError(e)) {
          sawNetworkError = true;
          consecutiveErrors += 1;
          if (consecutiveErrors <= maxNetworkRetries) {
            await sleep(retryDelayMs);
            continue;
          }
        }
        throw e;
      }
    }
  };
  
  const streams = [];
  for (let i = 0; i < numStreams; i++) {
    streams.push(uploadStream(i * streamDelay));
  }
  await Promise.all(streams);
  
  const overheadFactor = detectOverheadFactor();
  const endNow = Math.min(performance.now(), endTime);
  const actualMeasureStart = measureStartTime > 0 ? measureStartTime : startTime;
  const measureTime = Math.max(0.001, (endNow - actualMeasureStart) / 1000);
  const avgSpeed = (totalBytes * 8 * overheadFactor) / measureTime / 1_000_000;
  
  if (totalBytes === 0 && sawNetworkError) {
    throw new Error('Network error during upload. Try again or change server.');
  }

  return avgSpeed > 0 ? avgSpeed : 0;
}

function updateSpeed(speed, direction) {
  if (typeof speed !== 'number' || !Number.isFinite(speed) || speed < 0) speed = 0;
  if (!elements.speedNumber || !elements.speedUnit) return;
  state.currentSpeed = speed;
  
  let displaySpeed, unit;
  
  if (direction === 'latency') {
    displaySpeed = speed.toFixed(0);
    unit = 'ms';
    elements.speedNumber.className = 'speed-number measuring';
  } else if (speed >= 1000) {
    displaySpeed = (speed / 1000).toFixed(2);
    unit = 'Gbps';
    elements.speedNumber.className = 'speed-number ' + (direction === 'download' ? 'downloading' : 'uploading');
  } else {
    displaySpeed = speed.toFixed(1);
    unit = 'Mbps';
    elements.speedNumber.className = 'speed-number ' + (direction === 'download' ? 'downloading' : 'uploading');
  }
  
  elements.speedNumber.textContent = displaySpeed;
  elements.speedUnit.textContent = unit;
}

function updateProgress(progress) {
  if (!elements.progressRing) return;
  state.progress = progress;
  let offset = RING_CIRCUMFERENCE - (progress / 100) * RING_CIRCUMFERENCE;
  if (progress >= 99.5) {
    offset = -RING_END_OFFSET;
  }
  elements.progressRing.style.strokeDashoffset = offset;
}

function resetProgress() {
  if (!elements.progressRing || !elements.speedNumber) return;
  state.progress = 0;
  elements.progressRing.style.strokeDashoffset = RING_CIRCUMFERENCE;
  elements.speedNumber.textContent = '0';
}

function updateTestType(text, className) {
  if (!elements.testType || !elements.progressRing || !elements.speedNumber) return;
  elements.testType.textContent = text;
  elements.progressRing.className = 'progress-ring-fill ' + className;
  elements.speedNumber.className = 'speed-number ' + className;
}

function showState(stateName) {
  if (!elements.idleState || !elements.testingState || !elements.resultsState) return;
  elements.idleState.classList.add('hidden');
  elements.testingState.classList.add('hidden');
  elements.resultsState.classList.add('hidden');
  document.body.classList.toggle('results-view', stateName === 'results');
  
  switch (stateName) {
    case 'idle':
      elements.idleState.classList.remove('hidden');
      break;
    case 'testing':
      elements.testingState.classList.remove('hidden');
      break;
    case 'results':
      elements.resultsState.classList.remove('hidden');
      break;
  }
}

function showResults() {
  if (!elements.downloadResult || !elements.uploadResult || !elements.latencyResult || !elements.jitterResult) {
    return;
  }
  showState('results');
  
  const formatSpeedWithUnit = (speed) => {
    if (typeof speed !== 'number' || !Number.isFinite(speed) || speed < 0) speed = 0;
    if (speed >= 1000) {
      return { value: (speed / 1000).toFixed(2), unit: 'Gbps' };
    }
    return { value: speed.toFixed(1), unit: 'Mbps' };
  };
  
  const download = formatSpeedWithUnit(state.downloadResult);
  const upload = formatSpeedWithUnit(state.uploadResult);
  
  elements.downloadResult.textContent = download.value;
  elements.uploadResult.textContent = upload.value;
  
  const downloadUnit = document.querySelector('.result-primary .result-unit');
  const uploadUnit = document.querySelector('.result-secondary .result-unit');
  if (downloadUnit) downloadUnit.textContent = download.unit;
  if (uploadUnit) uploadUnit.textContent = upload.unit;
  
  elements.latencyResult.textContent = state.latencyResult != null ? `${state.latencyResult.toFixed(1)} ms` : '-';
  elements.jitterResult.textContent = state.jitterResult != null ? `${state.jitterResult.toFixed(1)} ms` : '-';
  
  // Loaded latency: use the worse of download/upload loaded latency
  const loadedLatency = Math.max(state.downloadLatency, state.uploadLatency);
  if (elements.loadedLatencyResult) {
    elements.loadedLatencyResult.textContent = loadedLatency > 0
      ? `${loadedLatency.toFixed(1)} ms`
      : '-';
  }
  
  // Bufferbloat grade based on latency increase under load
  if (elements.bufferbloatResult) {
    const grade = computeBufferbloatGrade(state.latencyResult, loadedLatency);
    elements.bufferbloatResult.textContent = grade || '-';
  }
  
  updateNetworkDisplay();
  
  saveAndEnableShare();
}

async function saveAndEnableShare() {
  const loadedLat = Math.max(state.downloadLatency, state.uploadLatency);
  const bbGrade = computeBufferbloatGrade(state.latencyResult, loadedLat) || '';

  try {
    const res = await fetch(`${apiBase}/results`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        download_mbps: state.downloadResult,
        upload_mbps: state.uploadResult,
        latency_ms: state.latencyResult || 0,
        jitter_ms: state.jitterResult || 0,
        loaded_latency_ms: loadedLat,
        bufferbloat_grade: bbGrade,
        ipv4: state.networkInfo.ipv4 || '',
        ipv6: state.networkInfo.ipv6 || '',
        server_name: resolveServerName()
      })
    });
    if (!res.ok) { await res.text().catch(() => {}); return; }
    const data = await res.json();
    if (typeof data?.id === 'string' && data.id.length > 0) {
      state.resultId = data.id;
    }
    if (state.resultId && elements.shareBtn) {
      elements.shareBtn.classList.remove('hidden');
    }
  } catch (err) {
    // Non-critical; share just won't be available
    console.debug('Share save unavailable:', err);
  }
}

function handleShare() {
  if (!state.resultId) return;
  const url = window.location.origin + '/results/' + state.resultId;
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(url).then(() => {
      showError('Link copied to clipboard', false);
    }).catch(() => {
      promptShareUrl(url);
    });
  } else {
    promptShareUrl(url);
  }
}

function promptShareUrl(url) {
  if (navigator.share) {
    navigator.share({ title: 'openByte Speed Test Result', url }).catch(() => {});
  } else {
    window.prompt('Copy this link:', url);
  }
}

function cancelTest() {
  if (state.abortController) {
    state.abortController.abort();
  }
  state.isRunning = false;
}

function resetToIdle() {
  cancelTest();
  
  state.phase = 'idle';
  state.currentSpeed = 0;
  state.progress = 0;
  state.downloadResult = 0;
  state.uploadResult = 0;
  state.latencyResult = null;
  state.jitterResult = null;
  state.downloadLatency = 0;
  state.uploadLatency = 0;
  state.resultId = null;
  if (elements.shareBtn) elements.shareBtn.classList.add('hidden');
  
  resetProgress();
  showState('idle');
  hideError();
}

function showError(message, isError = true) {
  if (!elements.errorToast || !elements.errorMessage) return;
  elements.errorMessage.textContent = message;
  const icon = elements.errorToast.querySelector('.toast-icon');
  if (toastTimer) {
    clearTimeout(toastTimer);
    toastTimer = null;
  }
  if (isError) {
    if (icon) icon.textContent = '⚠';
    elements.errorToast.classList.remove('hidden');
    elements.errorToast.style.background = '';
    toastTimer = setTimeout(hideError, 5000);
  } else {
    if (icon) icon.textContent = '✓';
    elements.errorToast.classList.remove('hidden');
    elements.errorToast.style.background = 'var(--accent-primary)';
    toastTimer = setTimeout(() => {
      hideError();
      elements.errorToast.style.background = '';
    }, 2000);
  }
}

function hideError() {
  if (!elements.errorToast) return;
  elements.errorToast.classList.add('hidden');
}

function notifySettingsSaved() {
  if (elements.settingsModal?.open) {
    showError('Settings saved', false);
  }
}
