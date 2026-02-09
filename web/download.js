(function() {
  const releaseUrl = 'https://api.github.com/repos/saveenergy/openbyte/releases/latest';
  const releasePage = 'https://github.com/saveenergy/openbyte/releases/latest';

  const platforms = {
    linux:   { suffixes: ['linux_amd64.tar.gz', 'linux_arm64.tar.gz'], icon: '🐧', name: 'Linux' },
    macos:   { suffixes: ['darwin_amd64.tar.gz', 'darwin_arm64.tar.gz'], icon: '🍎', name: 'macOS' },
    windows: { suffixes: ['windows_amd64.zip'], icon: '🪟', name: 'Windows' }
  };

  const archLabels = {
    'linux_amd64.tar.gz':   { arch: 'x86_64',       short: 'amd64' },
    'linux_arm64.tar.gz':   { arch: 'ARM64',         short: 'arm64' },
    'darwin_amd64.tar.gz':  { arch: 'Intel',         short: 'amd64' },
    'darwin_arm64.tar.gz':  { arch: 'Apple Silicon',  short: 'arm64' },
    'windows_amd64.zip':    { arch: 'x86_64',        short: 'amd64' }
  };

  function detectPlatform() {
    const ua = navigator.userAgent || '';
    // navigator.userAgentData is preferred (navigator.platform is deprecated)
    let platform = '';
    if (navigator.userAgentData && navigator.userAgentData.platform) {
      platform = navigator.userAgentData.platform;
    } else {
      platform = navigator.platform || '';
    }
    let os = 'linux', arch = 'amd64';

    if (/Mac/i.test(platform) || /Mac/i.test(ua)) {
      os = 'macos';
      // Apple Silicon detection
      if (/arm64/i.test(ua) || (typeof navigator.cpuClass === 'undefined' &&
          /Mac/.test(platform) && !(/Intel/.test(ua)))) {
        // Modern Macs — check via GL renderer if available
        try {
          const canvas = document.createElement('canvas');
          const gl = canvas.getContext('webgl');
          if (gl) {
            const dbg = gl.getExtension('WEBGL_debug_renderer_info');
            if (dbg) {
              const renderer = gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL);
              if (/Apple/.test(renderer) && !/Intel/.test(renderer)) arch = 'arm64';
            }
          }
        } catch(e) {}
      }
    } else if (/Win/i.test(platform) || /Win/i.test(ua)) {
      os = 'windows';
    } else {
      // Linux/other — check ARM
      if (/aarch64|arm64/i.test(ua)) arch = 'arm64';
    }
    return { os, arch };
  }

  function formatBytes(bytes) {
    if (!bytes) return '';
    const mb = bytes / (1024 * 1024);
    return mb.toFixed(1) + ' MB';
  }

  function findAsset(assets, suffix) {
    return assets.find(function(a) {
      return a.name.startsWith('openbyte_') && a.name.endsWith(suffix);
    });
  }

  function suffixForDetected(detected) {
    if (detected.os === 'windows') return 'windows_amd64.zip';
    const osKey = detected.os === 'macos' ? 'darwin' : 'linux';
    return osKey + '_' + detected.arch + (detected.os === 'windows' ? '.zip' : '.tar.gz');
  }

  function altSuffix(detected) {
    if (detected.os === 'windows') return null;
    const osKey = detected.os === 'macos' ? 'darwin' : 'linux';
    const other = detected.arch === 'arm64' ? 'amd64' : 'arm64';
    return osKey + '_' + other + '.tar.gz';
  }

  function renderRecommended(assets, detected, version) {
    const suffix = suffixForDetected(detected);
    const asset = findAsset(assets, suffix);
    const info = archLabels[suffix] || { arch: detected.arch, short: detected.arch };
    const platName = platforms[detected.os] ? platforms[detected.os].name : detected.os;

    const platformEl = document.getElementById('recommendedPlatform');
    const btn = document.getElementById('recommendedBtn');
    const label = document.getElementById('recommendedLabel');
    const meta = document.getElementById('recommendedMeta');

    platformEl.textContent = platName + ' · ' + info.arch;

    if (asset) {
      btn.href = asset.browser_download_url;
      btn.style.pointerEvents = '';
      btn.style.opacity = '';
      btn.rel = 'noopener noreferrer';
      label.textContent = 'Download ' + (version || '');
      meta.textContent = info.arch + ' · ' + formatBytes(asset.size);
    } else {
      btn.href = releasePage;
      btn.style.pointerEvents = '';
      btn.style.opacity = '';
      btn.rel = 'noopener noreferrer';
      label.textContent = 'View on GitHub';
      meta.textContent = '';
    }

    // Alt architecture link
    const alt = altSuffix(detected);
    const altEl = document.getElementById('altArch');
    if (alt) {
      const altAsset = findAsset(assets, alt);
      const altInfo = archLabels[alt];
      if (altAsset && altInfo) {
        const a = document.createElement('a');
        a.className = 'dl-alt-link';
        a.href = altAsset.browser_download_url;
        a.rel = 'noopener noreferrer';
        a.textContent = 'Also available for ' + altInfo.arch + ' (' + formatBytes(altAsset.size) + ')';
        altEl.appendChild(a);
      }
    }
  }

  function renderAllPlatforms(assets) {
    Object.keys(platforms).forEach(function(osKey) {
      const container = document.querySelector('[data-os="' + osKey + '"]');
      if (!container) return;
      while (container.firstChild) container.removeChild(container.firstChild);
      const plat = platforms[osKey];
      let found = false;

      plat.suffixes.forEach(function(suffix) {
        const asset = findAsset(assets, suffix);
        if (!asset) return;
        found = true;
        const info = archLabels[suffix] || { arch: suffix, short: suffix };

        const row = document.createElement('a');
        row.className = 'dl-asset-row';
        row.href = asset.browser_download_url;
        row.rel = 'noopener noreferrer';

        const nameSpan = document.createElement('span');
        nameSpan.className = 'dl-asset-name';
        nameSpan.textContent = info.arch;

        const sizeSpan = document.createElement('span');
        sizeSpan.className = 'dl-asset-size';
        sizeSpan.textContent = formatBytes(asset.size);

        row.appendChild(nameSpan);
        row.appendChild(sizeSpan);
        container.appendChild(row);
      });

      if (!found) {
        const fallback = document.createElement('a');
        fallback.className = 'download-link';
        fallback.href = releasePage;
        fallback.target = '_blank';
        fallback.rel = 'noopener noreferrer';
        fallback.textContent = 'View release';
        container.appendChild(fallback);
      }
    });
  }

  function renderInstall(detected, version) {
    const section = document.getElementById('installSection');
    const tabs = document.getElementById('installTabs');
    const cmd = document.getElementById('installCmd');

    let commands = {};
    const vTag = version || 'latest';

    commands['curl'] = 'curl -fsSL https://github.com/saveenergy/openbyte/releases/' +
      (version ? 'download/' + version : 'latest/download') +
      '/openbyte_' + (detected.os === 'macos' ? 'darwin' : 'linux') +
      '_' + detected.arch + '.tar.gz | tar xz';

    commands['docker'] = 'docker run -p 8080:8080 ghcr.io/saveenergy/openbyte:' +
      (version ? version.replace(/^v/, '') : 'latest') + ' server';

    if (detected.os === 'windows') {
      commands = {};
      commands['powershell'] = 'Invoke-WebRequest -Uri "https://github.com/saveenergy/openbyte/releases/' +
        (version ? 'download/' + version : 'latest/download') +
        '/openbyte_windows_amd64.zip" -OutFile openbyte.zip; Expand-Archive openbyte.zip';
      commands['docker'] = 'docker run -p 8080:8080 ghcr.io/saveenergy/openbyte:' +
        (version ? version.replace(/^v/, '') : 'latest') + ' server';
    }

    const keys = Object.keys(commands);
    if (keys.length === 0) return;
    section.style.display = '';

    let activeTab = keys[0];

    function renderTabs() {
      while (tabs.firstChild) tabs.removeChild(tabs.firstChild);
      keys.forEach(function(key) {
        const btn = document.createElement('button');
        btn.className = 'dl-install-tab' + (key === activeTab ? ' active' : '');
        btn.textContent = key;
        btn.onclick = function() {
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
    const tag = data.tag_name || '';
    const date = data.published_at ? new Date(data.published_at) : null;
    const el = document.getElementById('versionTag');
    const parts = [];
    if (tag) parts.push(tag);
    if (date) parts.push(date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' }));
    if (parts.length) el.textContent = ' · ' + parts.join(' · ');
  }

  function fallbackCopy(text) {
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.setAttribute('readonly', '');
    ta.style.position = 'absolute';
    ta.style.left = '-9999px';
    document.body.appendChild(ta);
    ta.select();
    let ok = false;
    try {
      ok = document.execCommand('copy');
    } catch (_) {}
    document.body.removeChild(ta);
    return ok;
  }

  function copyText(text, onSuccess, onFailure) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(onSuccess).catch(function() {
        if (fallbackCopy(text)) {
          onSuccess();
          return;
        }
        onFailure();
      });
      return;
    }

    if (fallbackCopy(text)) {
      onSuccess();
      return;
    }
    onFailure();
  }

  // Setup copy buttons
  function setupCopy(btnId, getTextFn) {
    const btn = document.getElementById(btnId);
    if (!btn) return;
    btn.addEventListener('click', function() {
      const text = getTextFn();
      copyText(text, function() {
        btn.textContent = 'Copied!';
        setTimeout(function() { btn.textContent = 'Copy'; }, 1500);
      }, function() {
        btn.textContent = 'Failed';
        setTimeout(function() { btn.textContent = 'Copy'; }, 1500);
      });
    });
  }

  setupCopy('copyBtn', function() {
    return document.getElementById('installCmd').textContent;
  });
  setupCopy('copyDockerBtn', function() {
    return document.querySelector('#copyDockerBtn').closest('.dl-code-block').querySelector('code').textContent;
  });

  const detected = detectPlatform();

  fetch(releaseUrl)
    .then(function(res) {
      if (!res.ok) {
        const reason = res.status === 403 ? 'GitHub API rate limited' : 'GitHub API error ' + res.status;
        throw new Error(reason);
      }
      return res.json();
    })
    .then(function(data) {
      const assets = Array.isArray(data.assets) ? data.assets : [];
      renderVersion(data);
      renderRecommended(assets, detected, data.tag_name);
      renderAllPlatforms(assets);
      renderInstall(detected, data.tag_name);
    })
    .catch(function(err) {
      console.warn('Release fetch failed:', err);
      const btn = document.getElementById('recommendedBtn');
      btn.href = releasePage;
      btn.style.pointerEvents = '';
      btn.style.opacity = '';
      document.getElementById('recommendedLabel').textContent = 'View on GitHub';
      document.getElementById('recommendedPlatform').textContent =
        err && err.message ? err.message : 'Could not load release data';
      document.querySelectorAll('.download-links').forEach(function(c) {
        while (c.firstChild) c.removeChild(c.firstChild);
        const link = document.createElement('a');
        link.className = 'download-link';
        link.href = releasePage;
        link.target = '_blank';
        link.rel = 'noopener noreferrer';
        link.textContent = 'View release';
        c.appendChild(link);
      });
    });
})();
