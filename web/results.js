(function() {
  var loadingView = document.getElementById('loadingView');
  var resultView = document.getElementById('resultView');
  var errorView = document.getElementById('errorView');
  if (!loadingView || !resultView || !errorView) {
    console.error('Results page missing required view elements');
    return;
  }

  var parts = window.location.pathname.replace(/\/+$/, '').split('/').filter(Boolean);
  var id = parts[parts.length - 1];
  if (!id || !/^[0-9a-zA-Z]{8}$/.test(id)) {
    showError();
    return;
  }

  loadingView.classList.remove('hidden');
  resultView.classList.add('hidden');

  fetch('/api/v1/results/' + id)
    .then(function(res) {
      if (!res.ok) {
        return res.text().catch(function() {}).then(function() {
          throw new Error('HTTP ' + res.status);
        });
      }
      return res.json();
    })
    .then(function(data) {
      loadingView.classList.add('hidden');
      resultView.classList.remove('hidden');
      renderResult(data);
    })
    .catch(function(err) {
      console.error('Results fetch failed:', err);
      showError();
    });

  function showError() {
    loadingView.classList.add('hidden');
    resultView.classList.add('hidden');
    errorView.classList.remove('hidden');
  }

  function formatSpeed(speed) {
    if (typeof speed !== 'number' || !Number.isFinite(speed) || speed < 0) speed = 0;
    if (speed >= 1000) return { value: (speed / 1000).toFixed(2), unit: 'Gbps' };
    return { value: speed.toFixed(1), unit: 'Mbps' };
  }

  function safeFixed(v, digits) {
    return typeof v === 'number' && Number.isFinite(v) ? v.toFixed(digits) : '-';
  }

  function renderResult(d) {
    if (!d || typeof d !== 'object') { showError(); return; }
    try {
      var downloadEl = document.getElementById('downloadResult');
      var uploadEl = document.getElementById('uploadResult');
      var latencyEl = document.getElementById('latencyResult');
      var jitterEl = document.getElementById('jitterResult');
      var loadedLatencyEl = document.getElementById('loadedLatencyResult');
      var bufferbloatEl = document.getElementById('bufferbloatResult');
      var ipv4El = document.getElementById('networkIPv4');
      var ipv6El = document.getElementById('networkIPv6');
      var serverLabelEl = document.getElementById('serverLabel');
      var serverItemEl = document.getElementById('serverItem');
      var serverValueEl = document.getElementById('serverValue');
      var testedAtEl = document.getElementById('testedAt');

      var dl = formatSpeed(typeof d.download_mbps === 'number' && Number.isFinite(d.download_mbps) ? d.download_mbps : 0);
      var ul = formatSpeed(typeof d.upload_mbps === 'number' && Number.isFinite(d.upload_mbps) ? d.upload_mbps : 0);

      if (downloadEl) downloadEl.textContent = dl.value;
      if (uploadEl) uploadEl.textContent = ul.value;

      var dlUnit = document.querySelector('.result-primary .result-unit');
      var ulUnit = document.querySelector('.result-secondary .result-unit');
      if (dlUnit) dlUnit.textContent = dl.unit;
      if (ulUnit) ulUnit.textContent = ul.unit;

      if (latencyEl) latencyEl.textContent = (typeof d.latency_ms === 'number' && d.latency_ms > 0) ? safeFixed(d.latency_ms, 1) + ' ms' : '-';
      if (jitterEl) jitterEl.textContent = (typeof d.jitter_ms === 'number' && d.jitter_ms > 0) ? safeFixed(d.jitter_ms, 1) + ' ms' : '-';
      if (loadedLatencyEl) loadedLatencyEl.textContent = (typeof d.loaded_latency_ms === 'number' && d.loaded_latency_ms > 0) ? safeFixed(d.loaded_latency_ms, 1) + ' ms' : '-';
      if (bufferbloatEl) {
        bufferbloatEl.textContent = (typeof d.bufferbloat_grade === 'string' && d.bufferbloat_grade) ? d.bufferbloat_grade : '-';
      }

      if (ipv4El) ipv4El.textContent = d.ipv4 || '-';
      if (ipv6El) ipv6El.textContent = d.ipv6 || '-';

      if (d.server_name && serverLabelEl && serverItemEl && serverValueEl) {
        serverLabelEl.style.display = '';
        serverItemEl.style.display = '';
        serverValueEl.textContent = typeof d.server_name === 'string' ? d.server_name : String(d.server_name);
      }

      if (d.created_at && testedAtEl) {
        try {
          var date = new Date(d.created_at);
          if (Number.isFinite(date.getTime())) {
            testedAtEl.textContent = date.toLocaleString();
          }
        } catch(_) {}
      }

      document.title = 'openByte — ' + dl.value + ' ' + dl.unit + ' / ' +
        ul.value + ' ' + ul.unit;
    } catch (_) { showError(); }
  }
})();
