(function() {
  var releaseUrl = 'https://api.github.com/repos/saveenergy/openbyte/releases/latest';
  var releasePage = 'https://github.com/saveenergy/openbyte/releases/latest';

  var platforms = {
    linux:   { suffixes: ['linux_amd64.tar.gz', 'linux_arm64.tar.gz'], icon: '🐧', name: 'Linux' },
    macos:   { suffixes: ['darwin_amd64.tar.gz', 'darwin_arm64.tar.gz'], icon: '🍎', name: 'macOS' },
    windows: { suffixes: ['windows_amd64.zip'], icon: '🪟', name: 'Windows' }
  };

  var archLabels = {
    'linux_amd64.tar.gz':   { arch: 'x86_64',       short: 'amd64' },
    'linux_arm64.tar.gz':   { arch: 'ARM64',         short: 'arm64' },
    'darwin_amd64.tar.gz':  { arch: 'Intel',         short: 'amd64' },
    'darwin_arm64.tar.gz':  { arch: 'Apple Silicon',  short: 'arm64' },
    'windows_amd64.zip':    { arch: 'x86_64',        short: 'amd64' }
  };

  function detectPlatform() {
    var ua = navigator.userAgent || '';
    var platform = navigator.platform || '';
    var os = 'linux', arch = 'amd64';

    if (/Mac/i.test(platform) || /Mac/i.test(ua)) {
      os = 'macos';
      // Apple Silicon detection
      if (/arm64/i.test(ua) || (typeof navigator.cpuClass === 'undefined' &&
          /Mac/.test(platform) && !(/Intel/.test(ua)))) {
        // Modern Macs — check via GL renderer if available
        try {
          var canvas = document.createElement('canvas');
          var gl = canvas.getContext('webgl');
          if (gl) {
            var dbg = gl.getExtension('WEBGL_debug_renderer_info');
            if (dbg) {
              var renderer = gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL);
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
    return { os: os, arch: arch };
  }

  function formatBytes(bytes) {
    if (!bytes) return '';
    var mb = bytes / (1024 * 1024);
    return mb.toFixed(1) + ' MB';
  }

  function findAsset(assets, suffix) {
    return assets.find(function(a) {
      return a.name.startsWith('openbyte_') && a.name.endsWith(suffix);
    });
  }

  function suffixForDetected(detected) {
    if (detected.os === 'windows') return 'windows_amd64.zip';
    var osKey = detected.os === 'macos' ? 'darwin' : 'linux';
    return osKey + '_' + detected.arch + (detected.os === 'windows' ? '.zip' : '.tar.gz');
  }

  function altSuffix(detected) {
    if (detected.os === 'windows') return null;
    var osKey = detected.os === 'macos' ? 'darwin' : 'linux';
    var other = detected.arch === 'arm64' ? 'amd64' : 'arm64';
    return osKey + '_' + other + '.tar.gz';
  }

  function renderRecommended(assets, detected, version) {
    var suffix = suffixForDetected(detected);
    var asset = findAsset(assets, suffix);
    var info = archLabels[suffix] || { arch: detected.arch, short: detected.arch };
    var platName = platforms[detected.os] ? platforms[detected.os].name : detected.os;

    var platformEl = document.getElementById('recommendedPlatform');
    var btn = document.getElementById('recommendedBtn');
    var label = document.getElementById('recommendedLabel');
    var meta = document.getElementById('recommendedMeta');

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
    var alt = altSuffix(detected);
    var altEl = document.getElementById('altArch');
    if (alt) {
      var altAsset = findAsset(assets, alt);
      var altInfo = archLabels[alt];
      if (altAsset && altInfo) {
        var a = document.createElement('a');
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
      var container = document.querySelector('[data-os="' + osKey + '"]');
      if (!container) return;
      container.innerHTML = '';
      var plat = platforms[osKey];
      var found = false;

      plat.suffixes.forEach(function(suffix) {
        var asset = findAsset(assets, suffix);
        if (!asset) return;
        found = true;
        var info = archLabels[suffix] || { arch: suffix, short: suffix };

        var row = document.createElement('a');
        row.className = 'dl-asset-row';
        row.href = asset.browser_download_url;
        row.rel = 'noopener noreferrer';

        var nameSpan = document.createElement('span');
        nameSpan.className = 'dl-asset-name';
        nameSpan.textContent = info.arch;

        var sizeSpan = document.createElement('span');
        sizeSpan.className = 'dl-asset-size';
        sizeSpan.textContent = formatBytes(asset.size);

        row.appendChild(nameSpan);
        row.appendChild(sizeSpan);
        container.appendChild(row);
      });

      if (!found) {
        var fallback = document.createElement('a');
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
    var section = document.getElementById('installSection');
    var tabs = document.getElementById('installTabs');
    var cmd = document.getElementById('installCmd');

    var commands = {};
    var vTag = version || 'latest';

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

    var keys = Object.keys(commands);
    if (keys.length === 0) return;
    section.style.display = '';

    var activeTab = keys[0];

    function renderTabs() {
      tabs.innerHTML = '';
      keys.forEach(function(key) {
        var btn = document.createElement('button');
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
    var tag = data.tag_name || '';
    var date = data.published_at ? new Date(data.published_at) : null;
    var el = document.getElementById('versionTag');
    var parts = [];
    if (tag) parts.push(tag);
    if (date) parts.push(date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' }));
    if (parts.length) el.textContent = ' · ' + parts.join(' · ');
  }

  // Setup copy buttons
  function setupCopy(btnId, getTextFn) {
    var btn = document.getElementById(btnId);
    if (!btn) return;
    btn.addEventListener('click', function() {
      var text = getTextFn();
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(function() {
          btn.textContent = 'Copied!';
          setTimeout(function() { btn.textContent = 'Copy'; }, 1500);
        }).catch(function() {
          btn.textContent = 'Failed';
          setTimeout(function() { btn.textContent = 'Copy'; }, 1500);
        });
      }
    });
  }

  setupCopy('copyBtn', function() {
    return document.getElementById('installCmd').textContent;
  });
  setupCopy('copyDockerBtn', function() {
    return document.querySelector('#copyDockerBtn').closest('.dl-code-block').querySelector('code').textContent;
  });

  var detected = detectPlatform();

  fetch(releaseUrl)
    .then(function(res) {
      if (!res.ok) {
        var reason = res.status === 403 ? 'GitHub API rate limited' : 'GitHub API error ' + res.status;
        throw new Error(reason);
      }
      return res.json();
    })
    .then(function(data) {
      var assets = Array.isArray(data.assets) ? data.assets : [];
      renderVersion(data);
      renderRecommended(assets, detected, data.tag_name);
      renderAllPlatforms(assets);
      renderInstall(detected, data.tag_name);
    })
    .catch(function(err) {
      console.warn('Release fetch failed:', err);
      var btn = document.getElementById('recommendedBtn');
      btn.href = releasePage;
      btn.style.pointerEvents = '';
      btn.style.opacity = '';
      document.getElementById('recommendedLabel').textContent = 'View on GitHub';
      document.getElementById('recommendedPlatform').textContent =
        err && err.message ? err.message : 'Could not load release data';
      document.querySelectorAll('.download-links').forEach(function(c) {
        c.innerHTML = '<a class="download-link" href="' + releasePage +
          '" target="_blank" rel="noopener noreferrer">View release</a>';
      });
    });
})();
