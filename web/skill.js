(function() {
  function copyText(text, btn) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(function() {
        btn.textContent = 'Copied!';
        setTimeout(function() { btn.textContent = 'Copy'; }, 1500);
      }).catch(function() {
        btn.textContent = 'Failed';
        setTimeout(function() { btn.textContent = 'Copy'; }, 1500);
      });
    }
  }

  function setupCopy(btnId) {
    var btn = document.getElementById(btnId);
    if (!btn) return;
    btn.addEventListener('click', function() {
      var code = btn.closest('.dl-code-block').querySelector('code');
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
