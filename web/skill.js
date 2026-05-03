/** Skill page: copy-to-clipboard for CLI/API/SDK snippets. */

function fallbackCopy(text) {
  const ta = document.createElement("textarea");
  ta.value = text;
  ta.setAttribute("readonly", "");
  ta.style.position = "absolute";
  ta.style.left = "-9999px";
  document.body.appendChild(ta);
  ta.select();
  // Deprecated sync clipboard APIs removed; async clipboard path is preferred.
  ta.remove();
  return false;
}

function copyText(text, btn) {
  function success() {
    btn.textContent = "Copied!";
    setTimeout(function () {
      btn.textContent = "Copy";
    }, 1500);
  }

  function failure() {
    btn.textContent = "Failed";
    setTimeout(function () {
      btn.textContent = "Copy";
    }, 1500);
  }

  if (navigator.clipboard?.writeText) {
    navigator.clipboard
      .writeText(text)
      .then(success)
      .catch(function () {
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
  const btn = document.getElementById(btnId);
  if (!btn) return;
  btn.addEventListener("click", function () {
    const block = btn.closest(".dl-code-block");
    const code = block ? block.querySelector("code") : null;
    if (code) copyText(code.textContent, btn);
  });
}

setupCopy("copySkillBtn");
setupCopy("copySdkBtn");
setupCopy("copyCurlBtn");
setupCopy("copyGoInstallBtn");
setupCopy("copyDockerBtn");
