(function() {
  function fallbackCopy(text) {
    var ta = document.createElement('textarea');
    ta.value = text;
    ta.setAttribute('readonly', '');
    ta.style.position = 'absolute';
    ta.style.left = '-9999px';
    document.body.appendChild(ta);
    ta.select();
    var ok = false;
    try {
      ok = document.execCommand('copy');
    } catch (_) {}
    document.body.removeChild(ta);
    return ok;
  }

  function copyText(text, btn) {
    function success() {
      btn.textContent = 'Copied!';
      setTimeout(function() { btn.textContent = 'Copy'; }, 1500);
    }

    function failure() {
      btn.textContent = 'Failed';
      setTimeout(function() { btn.textContent = 'Copy'; }, 1500);
    }

    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(success).catch(function() {
        if (fallbackCopy(text)) {
          success();
          return;
        }
        failure();
      });
      return;
    }

    if (fallbackCopy(text)) {
      success();
      return;
    }
    failure();
  }

  function setupCopy(btnId) {
    var btn = document.getElementById(btnId);
    if (!btn) return;
    btn.addEventListener('click', function() {
      var block = btn.closest('.dl-code-block');
      var code = block ? block.querySelector('code') : null;
      if (code) copyText(code.textContent, btn);
    });
  }

  setupCopy('copySkillBtn');
  setupCopy('copyMcpBtn');
  setupCopy('copySdkBtn');
  setupCopy('copyCurlBtn');
  setupCopy('copyGoInstallBtn');
  setupCopy('copyDockerBtn');
})();
