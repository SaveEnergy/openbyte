(function() {
  var parts = window.location.pathname.replace(/\/+$/, '').split('/').filter(Boolean);
  var id = parts[parts.length - 1];
  if (!id || !/^[0-9a-zA-Z]{8}$/.test(id)) {
    showError();
    return;
  }

  document.getElementById('loadingView').classList.remove('hidden');
  document.getElementById('resultView').classList.add('hidden');

  fetch('/api/v1/results/' + id)
    .then(function(res) {
      if (!res.ok) throw new Error('HTTP ' + res.status);
      return res.json();
    })
    .then(function(data) {
      document.getElementById('loadingView').classList.add('hidden');
      document.getElementById('resultView').classList.remove('hidden');
      renderResult(data);
    })
    .catch(function(err) {
      console.error('Results fetch failed:', err);
      showError();
    });

  function showError() {
    document.getElementById('loadingView').classList.add('hidden');
    document.getElementById('resultView').classList.add('hidden');
    document.getElementById('errorView').classList.remove('hidden');
  }

  function formatSpeed(speed) {
    if (typeof speed !== 'number' || !isFinite(speed) || speed < 0) speed = 0;
    if (speed >= 1000) return { value: (speed / 1000).toFixed(2), unit: 'Gbps' };
    return { value: speed.toFixed(1), unit: 'Mbps' };
  }

  function safeFixed(v, digits) {
    return typeof v === 'number' && isFinite(v) ? v.toFixed(digits) : '-';
  }

  function renderResult(d) {
    if (!d || typeof d !== 'object') { showError(); return; }
    try {
      var dl = formatSpeed(typeof d.download_mbps === 'number' && isFinite(d.download_mbps) ? d.download_mbps : 0);
      var ul = formatSpeed(typeof d.upload_mbps === 'number' && isFinite(d.upload_mbps) ? d.upload_mbps : 0);

      document.getElementById('downloadResult').textContent = dl.value;
      document.getElementById('uploadResult').textContent = ul.value;

      var dlUnit = document.querySelector('.result-primary .result-unit');
      var ulUnit = document.querySelector('.result-secondary .result-unit');
      if (dlUnit) dlUnit.textContent = dl.unit;
      if (ulUnit) ulUnit.textContent = ul.unit;

      document.getElementById('latencyResult').textContent =
        d.latency_ms > 0 ? safeFixed(d.latency_ms, 1) + ' ms' : '-';
      document.getElementById('jitterResult').textContent =
        d.jitter_ms > 0 ? safeFixed(d.jitter_ms, 1) + ' ms' : '-';
      document.getElementById('loadedLatencyResult').textContent =
        d.loaded_latency_ms > 0 ? safeFixed(d.loaded_latency_ms, 1) + ' ms' : '-';
      document.getElementById('bufferbloatResult').textContent =
        d.bufferbloat_grade || '-';

      document.getElementById('networkIPv4').textContent = d.ipv4 || '-';
      document.getElementById('networkIPv6').textContent = d.ipv6 || '-';

      if (d.server_name) {
        document.getElementById('serverLabel').style.display = '';
        document.getElementById('serverItem').style.display = '';
        document.getElementById('serverValue').textContent = d.server_name;
      }

      if (d.created_at) {
        try {
          var date = new Date(d.created_at);
          document.getElementById('testedAt').textContent = date.toLocaleString();
        } catch(_) {}
      }

      document.title = 'openByte — ' + dl.value + ' ' + dl.unit + ' / ' +
        ul.value + ' ' + ul.unit;
    } catch (_) { showError(); }
  }
})();
