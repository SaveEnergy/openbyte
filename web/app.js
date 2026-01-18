let apiBase = '/api/v1';

const state = {
  phase: 'idle',
  isRunning: false,
  downloadResult: 0,
  uploadResult: 0,
  latencyResult: 0,
  jitterResult: 0,
  currentSpeed: 0,
  progress: 0,
  ws: null,
  streamId: null,
  abortController: null,
  servers: [],
  selectedServer: null,
  settings: {
    duration: 10,
    streams: 8,  // Default 8 streams for ~600 Mbps capacity
    serverUrl: ''
  },
  networkInfo: {
    connectionType: null,
    ipv6: false,
    clientIP: null
  },
  rttMetrics: {
    baseline: null,
    current: null,
    avg: null,
    jitter: null
  },
  latencyPercentiles: {
    p50: null,
    p95: null,
    p99: null
  }
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
  latencyP50: document.getElementById('latencyP50'),
  latencyP95: document.getElementById('latencyP95'),
  latencyP99: document.getElementById('latencyP99'),
  rttBaseline: document.getElementById('rttBaseline'),
  rttCurrent: document.getElementById('rttCurrent'),
  rttAvg: document.getElementById('rttAvg'),
  rttJitter: document.getElementById('rttJitter'),
  networkType: document.getElementById('networkType'),
  networkIPv6: document.getElementById('networkIPv6'),
  networkIP: document.getElementById('networkIP'),
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
  errorMessage: document.getElementById('errorMessage')
};

const RING_CIRCUMFERENCE = 2 * Math.PI * 90;
const RING_END_OFFSET = 2;
let lastModalTrigger = null;
let toastTimer = null;

document.addEventListener('DOMContentLoaded', () => {
  loadSettings();
  loadServers();
  bindEvents();
});

function bindEvents() {
  elements.startBtn.addEventListener('click', startTest);
  elements.restartBtn.addEventListener('click', resetToIdle);
  if (elements.cancelBtn) {
    elements.cancelBtn.addEventListener('click', resetToIdle);
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
      state.settings = { ...state.settings, ...s };
      elements.duration.value = state.settings.duration;
      elements.streams.value = state.settings.streams;
      if (state.settings.serverUrl && elements.customServerUrl) {
        elements.customServerUrl.value = state.settings.serverUrl;
      }
    } catch (e) {}
  }
}

function saveSettings() {
  state.settings.duration = parseInt(elements.duration.value);
  state.settings.streams = parseInt(elements.streams.value);
  localStorage.setItem('obyte-settings', JSON.stringify(state.settings));
  notifySettingsSaved();
}

function detectNetworkInfo() {
  if (navigator.connection) {
    const conn = navigator.connection;
    const typeMap = {
      'ethernet': 'Ethernet',
      'wifi': 'WiFi',
      'cellular': 'Cellular',
      'wimax': 'WiMAX',
      'bluetooth': 'Bluetooth',
      'other': 'Other'
    };
    
    state.networkInfo.connectionType = typeMap[conn.type] || null;
    
    if (elements.networkType) {
      elements.networkType.textContent = state.networkInfo.connectionType || 'Unknown';
    }
  } else {
    if (elements.networkType) {
      elements.networkType.textContent = 'Unknown';
    }
  }
  
  fetch(`${apiBase}/ping`)
    .then(res => res.json())
    .then(data => {
      if (data.client_ip) {
        state.networkInfo.clientIP = data.client_ip;
        state.networkInfo.ipv6 = data.ipv6 || false;
        
        if (elements.networkIP) {
          elements.networkIP.textContent = data.client_ip;
        }
        if (elements.networkIPv6) {
          elements.networkIPv6.textContent = data.ipv6 ? 'Yes' : 'No';
        }
      }
    })
    .catch(() => {
      if (elements.networkIPv6) {
        elements.networkIPv6.textContent = 'Unknown';
      }
    });
}

async function loadServers() {
  try {
    const res = await fetch(`${apiBase}/servers`);
    const data = await res.json();
    state.servers = data.servers || [];
    populateServerSelect();
    
    if (state.servers.length > 0) {
      await selectFastestServer();
    } else {
      checkServer();
    }
  } catch (e) {
    console.error('Failed to load servers:', e);
    checkServer();
  }
}

async function selectFastestServer() {
  if (state.servers.length === 0) {
    checkServer();
    return;
  }

  if (state.servers.length === 1) {
    state.selectedServer = state.servers[0];
    checkServer();
    return;
  }

  elements.serverText.textContent = 'Finding fastest...';

  const latencyPromises = state.servers.map(async (server) => {
    const healthUrl = server.api_endpoint 
      ? `${server.api_endpoint}/health`
      : `http://${server.host}:8080/health`;
    
    const start = performance.now();
    try {
      const res = await fetch(healthUrl, { 
        method: 'GET',
        mode: 'cors',
        signal: AbortSignal.timeout(5000)
      });
      const latency = performance.now() - start;
      
      if (res.ok) {
        return { server, latency, error: null };
      }
      return { server, latency: Infinity, error: 'unhealthy' };
    } catch (e) {
      return { server, latency: Infinity, error: e.message };
    }
  });

  const results = await Promise.all(latencyPromises);
  
  const reachable = results.filter(r => r.error === null);
  
  if (reachable.length === 0) {
    console.warn('No servers reachable, using first available');
    state.selectedServer = state.servers[0];
    checkServer();
    return;
  }

  reachable.sort((a, b) => a.latency - b.latency);
  state.selectedServer = reachable[0].server;
  
  console.log('Auto-selected server:', state.selectedServer.name, 
    `(${Math.round(reachable[0].latency)}ms)`);
  
  checkServer();
}

function populateServerSelect() {
  if (!elements.serverSelect) return;
  
  elements.serverSelect.innerHTML = '';
  
  const currentOpt = document.createElement('option');
  currentOpt.value = 'current';
  currentOpt.textContent = 'Current Server';
  elements.serverSelect.appendChild(currentOpt);
  
  state.servers.forEach(server => {
    const opt = document.createElement('option');
    opt.value = server.id;
    opt.textContent = `${server.name} (${server.location})`;
    elements.serverSelect.appendChild(opt);
  });
  
  const customOpt = document.createElement('option');
  customOpt.value = 'custom';
  customOpt.textContent = 'Custom Server...';
  elements.serverSelect.appendChild(customOpt);
  
  if (state.settings.serverUrl) {
    elements.serverSelect.value = 'custom';
    showCustomServerInput(true);
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
  
  saveSettings();
  checkServer();
}

function onCustomServerChange() {
  const url = elements.customServerUrl.value.trim();
  if (url) {
    let serverUrl = url;
    if (!serverUrl.startsWith('http://') && !serverUrl.startsWith('https://')) {
      serverUrl = 'http://' + serverUrl;
    }
    apiBase = `${serverUrl}/api/v1`;
    state.settings.serverUrl = serverUrl;
    state.selectedServer = { id: 'custom', name: 'Custom', location: url, api_endpoint: serverUrl };
    saveSettings();
    checkServer();
  }
}

function showCustomServerInput(show) {
  const container = elements.customServerUrl?.parentElement;
  if (container) {
    container.style.display = show ? 'block' : 'none';
  }
}

async function checkServer() {
  const healthUrl = state.settings.serverUrl ? `${state.settings.serverUrl}/health` : '/health';
  
  try {
    const res = await fetch(healthUrl);
    const data = await res.json();
    if (data.status === 'ok' || data.status === 'healthy') {
      elements.serverDot.classList.remove('error');
      elements.serverDot.classList.add('connected');
      
      if (state.selectedServer) {
        elements.serverText.textContent = state.selectedServer.name || 'Ready';
      } else {
        elements.serverText.textContent = 'Ready';
      }
      
      if (elements.serverStatus) {
        elements.serverStatus.textContent = 'Connected';
        elements.serverStatus.className = 'server-status connected';
      }
    } else {
      throw new Error('Server degraded');
    }
  } catch (e) {
    elements.serverDot.classList.remove('connected');
    elements.serverDot.classList.add('error');
    elements.serverText.textContent = 'Offline';
    
    if (elements.serverStatus) {
      elements.serverStatus.textContent = 'Offline';
      elements.serverStatus.className = 'server-status error';
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
  
  try {
    state.phase = 'latency';
    showState('testing');
    updateTestType('◎ Latency', 'measuring');
    
    const latency = await measureLatency();
    state.latencyResult = latency;
    state.rttMetrics.baseline = latency;
    state.rttMetrics.avg = latency;
    
    if (!state.isRunning) return;
    
    state.phase = 'download';
    resetProgress();
    updateTestType('↓ Download', 'downloading');
    
    // Yield to render UI before starting test
    await new Promise(r => requestAnimationFrame(r));
    
    const downloadSpeed = await runTest('download');
    state.downloadResult = downloadSpeed;
    
    if (!state.isRunning) return;
    
    state.phase = 'upload';
    resetProgress();
    updateTestType('↑ Upload', 'uploading');
    
    // Yield to render UI before starting test
    await new Promise(r => requestAnimationFrame(r));
    
    const uploadSpeed = await runTest('upload');
    state.uploadResult = uploadSpeed;
    
    state.phase = 'results';
    showResults();
    
  } catch (e) {
    console.error('Test failed:', e);
    if (e.name !== 'AbortError') {
      showError(e.message || 'Test failed');
    }
    resetToIdle();
  } finally {
    state.isRunning = false;
    state.abortController = null;
  }
}

async function runTest(direction) {
  if (!state.isRunning) {
    throw new Error('Test cancelled');
  }
  
  const duration = state.settings.duration;
  const startTime = performance.now();
  let lastUpdate = startTime;
  let lastBytes = 0;
  
  const onProgress = (bytes, elapsed) => {
    const now = performance.now();
    const intervalMs = now - lastUpdate;
    
    if (intervalMs >= 200) {
      const intervalBytes = bytes - lastBytes;
      
      // Only update if we have positive progress
      if (intervalBytes > 0 && intervalMs > 0) {
        const instantSpeed = (intervalBytes * 8) / (intervalMs / 1000) / 1_000_000;
        updateSpeed(Math.max(0, instantSpeed), direction);
      }
      
      updateProgress(Math.min(100, (elapsed / duration) * 100));
      
      lastUpdate = now;
      lastBytes = Math.max(lastBytes, bytes); // Never decrease lastBytes
    }
  };
  
  let result;
  if (direction === 'download') {
    result = await runDownloadTest(duration, onProgress);
  } else {
    result = await runUploadTest(duration, onProgress);
  }

  if (state.isRunning) {
    updateProgress(100);
  }

  return result;
}

async function measureLatency() {
  const samples = [];
  const numSamples = 20;
  
  for (let i = 0; i < numSamples; i++) {
    if (!state.isRunning) break;
    
    const start = performance.now();
    try {
      await fetch(`${apiBase}/ping`, { 
        method: 'GET',
        signal: state.abortController?.signal 
      });
      const rtt = performance.now() - start;
      samples.push(rtt);
      
      updateProgress((i / numSamples) * 100);
      updateSpeed(rtt, 'latency');
    } catch (e) {
      if (e.name === 'AbortError') break;
    }
  }
  
  if (samples.length === 0) return 0;
  
  samples.sort((a, b) => a - b);
  const median = samples[Math.floor(samples.length / 2)];
  
  state.latencyPercentiles.p50 = samples[Math.floor(samples.length * 0.5)];
  state.latencyPercentiles.p95 = samples[Math.floor(samples.length * 0.95)] || samples[samples.length - 1];
  state.latencyPercentiles.p99 = samples[Math.floor(samples.length * 0.99)] || samples[samples.length - 1];
  
  if (samples.length >= 2) {
    let sumDiff = 0;
    for (let i = 1; i < samples.length; i++) {
      sumDiff += Math.abs(samples[i] - samples[i-1]);
    }
    state.jitterResult = sumDiff / (samples.length - 1);
  }
  
  return median;
}

// HTTP-based download test
async function runDownloadTest(duration, onProgress) {
  const startTime = performance.now();
  const numStreams = state.settings.streams || 8;
  const graceTime = 1500;
  const overheadFactor = 1.06;
  const streamDelay = 200;
  const endTime = startTime + (duration * 1000);
  let totalBytes = 0;
  let graceBytes = 0;
  let graceComplete = false;
  
  const streamPromises = [];
  
  for (let i = 0; i < numStreams; i++) {
    const delay = i * streamDelay;
    
    const streamPromise = (async () => {
      await new Promise(r => setTimeout(r, delay));
      
      try {
        const res = await fetch(`${apiBase}/download?duration=${duration}&chunk=1048576`, {
          method: 'GET',
          signal: state.abortController?.signal
        });
        
        if (!res.ok) return 0;
        
        const reader = res.body.getReader();
        
        while (true) {
          if (!state.isRunning) break;

          const elapsed = performance.now() - startTime;
          if (elapsed >= (endTime - startTime)) {
            await reader.cancel();
            break;
          }

          const { done, value } = await reader.read();
          if (done) break;
          
          if (!graceComplete && elapsed < graceTime) {
            graceBytes += value.length;
          } else {
            if (!graceComplete) {
              graceComplete = true;
              totalBytes = 0;
            }
            totalBytes += value.length;
          }
          
          const elapsedSec = (performance.now() - startTime) / 1000;
          const displayBytes = graceComplete ? totalBytes : graceBytes;
          onProgress(displayBytes, elapsedSec);
        }
        
        reader.releaseLock();
      } catch (e) {
        if (e.name !== 'AbortError') {
          console.warn('Download stream error:', e.message);
        }
      }
    })();
    
    streamPromises.push(streamPromise);
  }
  
  await Promise.all(streamPromises);
  
  const measuredElapsed = Math.min(performance.now(), endTime) - startTime;
  const measureTime = Math.max(0.001, (measuredElapsed - graceTime) / 1000);
  const avgSpeed = (totalBytes * 8 * overheadFactor) / measureTime / 1_000_000;
  
  return avgSpeed > 0 ? avgSpeed : 0;
}

async function runUploadTest(duration, onProgress) {
  const startTime = performance.now();
  const numStreams = state.settings.streams || 8;
  const graceTime = 3000;
  const overheadFactor = 1.06;
  const streamDelay = 200;
  const blobSize = 4 * 1024 * 1024;
  let totalBytes = 0;
  let graceBytes = 0;
  let graceComplete = false;
  
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
    
    while (performance.now() < endTime && state.isRunning) {
      try {
        const res = await fetch(`${apiBase}/upload`, {
          method: 'POST',
          body: blob,
          headers: { 'Content-Type': 'application/octet-stream' },
          signal: state.abortController?.signal
        });
        
        if (!res.ok) continue;
        
        const elapsed = performance.now() - startTime;
        
        if (!graceComplete && elapsed < graceTime) {
          graceBytes += blobSize;
        } else {
          if (!graceComplete) {
            graceComplete = true;
            totalBytes = 0;
          }
          totalBytes += blobSize;
        }
        
        const elapsedSec = (performance.now() - startTime) / 1000;
        const displayBytes = graceComplete ? totalBytes : graceBytes;
        onProgress(displayBytes, elapsedSec);
        
      } catch (e) {
        if (e.name === 'AbortError') break;
      }
    }
  };
  
  const streams = [];
  for (let i = 0; i < numStreams; i++) {
    streams.push(uploadStream(i * streamDelay));
  }
  await Promise.all(streams);
  
  const measuredElapsed = Math.min(performance.now(), endTime) - startTime;
  const measureTime = Math.max(0.001, (measuredElapsed - graceTime) / 1000);
  const avgSpeed = (totalBytes * 8 * overheadFactor) / measureTime / 1_000_000;
  
  return avgSpeed > 0 ? avgSpeed : 0;
}

function updateSpeed(speed, direction) {
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
  state.progress = progress;
  let offset = RING_CIRCUMFERENCE - (progress / 100) * RING_CIRCUMFERENCE;
  if (progress >= 99.5) {
    offset = -RING_END_OFFSET;
  }
  elements.progressRing.style.strokeDashoffset = offset;
}

function resetProgress() {
  state.progress = 0;
  elements.progressRing.style.strokeDashoffset = RING_CIRCUMFERENCE;
  elements.speedNumber.textContent = '0';
}

function updateTestType(text, className) {
  elements.testType.textContent = text;
  elements.progressRing.className = 'progress-ring-fill ' + className;
  elements.speedNumber.className = 'speed-number ' + className;
}

function showState(stateName) {
  elements.idleState.classList.add('hidden');
  elements.testingState.classList.add('hidden');
  elements.resultsState.classList.add('hidden');
  
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
  showState('results');
  
  const formatSpeedWithUnit = (speed) => {
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
  
  elements.latencyResult.textContent = `${state.latencyResult.toFixed(1)} ms`;
  elements.jitterResult.textContent = `${state.jitterResult.toFixed(1)} ms`;
  
  if (elements.latencyP50) {
    elements.latencyP50.textContent = state.latencyPercentiles.p50 !== null 
      ? `${state.latencyPercentiles.p50.toFixed(2)} ms` : '-';
  }
  if (elements.latencyP95) {
    elements.latencyP95.textContent = state.latencyPercentiles.p95 !== null 
      ? `${state.latencyPercentiles.p95.toFixed(2)} ms` : '-';
  }
  if (elements.latencyP99) {
    elements.latencyP99.textContent = state.latencyPercentiles.p99 !== null 
      ? `${state.latencyPercentiles.p99.toFixed(2)} ms` : '-';
  }
  
  if (elements.rttBaseline) {
    elements.rttBaseline.textContent = state.rttMetrics.baseline !== null 
      ? `${state.rttMetrics.baseline.toFixed(2)} ms` : '-';
  }
  if (elements.rttCurrent) {
    elements.rttCurrent.textContent = state.rttMetrics.current !== null 
      ? `${state.rttMetrics.current.toFixed(2)} ms` : '-';
  }
  if (elements.rttAvg) {
    elements.rttAvg.textContent = state.rttMetrics.avg !== null 
      ? `${state.rttMetrics.avg.toFixed(2)} ms` : '-';
  }
  if (elements.rttJitter) {
    elements.rttJitter.textContent = state.rttMetrics.jitter !== null 
      ? `${state.rttMetrics.jitter.toFixed(2)} ms` : '-';
  }
  
  if (elements.networkType) {
    elements.networkType.textContent = state.networkInfo.connectionType || 'Unknown';
  }
  if (elements.networkIPv6) {
    elements.networkIPv6.textContent = state.networkInfo.ipv6 ? 'Yes' : 'No';
  }
  
}

function cancelTest() {
  if (state.abortController) {
    state.abortController.abort();
  }
  if (state.ws) {
    state.ws.close();
    state.ws = null;
  }
  if (state.streamId) {
    fetch(`${apiBase}/stream/${state.streamId}/cancel`, { method: 'POST' }).catch(() => {});
  }
  state.isRunning = false;
  state.streamId = null;
}

function resetToIdle() {
  cancelTest();
  
  state.phase = 'idle';
  state.currentSpeed = 0;
  state.progress = 0;
  state.downloadResult = 0;
  state.uploadResult = 0;
  state.latencyResult = 0;
  state.jitterResult = 0;
  state.rttMetrics = { baseline: null, current: null, avg: null, jitter: null };
  state.latencyPercentiles = { p50: null, p95: null, p99: null };
  
  resetProgress();
  showState('idle');
  hideError();
}

function showError(message, isError = true) {
  elements.errorMessage.textContent = message;
  if (toastTimer) {
    clearTimeout(toastTimer);
    toastTimer = null;
  }
  if (isError) {
    elements.errorToast.classList.remove('hidden');
    elements.errorToast.style.background = '';
    toastTimer = setTimeout(hideError, 5000);
  } else {
    elements.errorToast.classList.remove('hidden');
    elements.errorToast.style.background = 'var(--accent-primary)';
    toastTimer = setTimeout(() => {
      hideError();
      elements.errorToast.style.background = '';
    }, 2000);
  }
}

function hideError() {
  elements.errorToast.classList.add('hidden');
}

function notifySettingsSaved() {
  if (elements.settingsModal?.open) {
    showError('Settings saved', false);
  }
}
