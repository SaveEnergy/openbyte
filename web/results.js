(function () {
  const loadingView = document.getElementById("loadingView");
  const resultView = document.getElementById("resultView");
  const errorView = document.getElementById("errorView");
  const errorMessage = document.querySelector("#errorView .error-message");
  if (!loadingView || !resultView || !errorView) {
    console.error("Results page missing required view elements");
    return;
  }

  const parts = globalThis.location.pathname
    .replace(/\/+$/, "")
    .split("/")
    .filter(Boolean);
  const id = parts.at(-1);
  if (!id || !/^[0-9a-zA-Z]{8}$/.test(id)) {
    showError("Result ID format is invalid.");
    return;
  }

  loadingView.classList.remove("hidden");
  resultView.classList.add("hidden");

  function userError(message) {
    const err = new Error(message);
    err.userSafe = true;
    return err;
  }

  const timeoutController = new AbortController();
  const timeoutHandle = setTimeout(function () {
    timeoutController.abort();
  }, 20000);

  loadResult(id, timeoutController.signal)
    .then(function (data) {
      clearTimeout(timeoutHandle);
      loadingView.classList.add("hidden");
      resultView.classList.remove("hidden");
      renderResult(data);
    })
    .catch(function (err) {
      clearTimeout(timeoutHandle);
      console.error("Results fetch failed:", err);
      if (err?.name === "AbortError") {
        showError("Request timed out. Please try again.");
        return;
      }
      if (err?.userSafe && err?.message) {
        showError(err.message);
        return;
      }
      showError("Unable to load result.");
    });

  function showError(message) {
    loadingView.classList.add("hidden");
    resultView.classList.add("hidden");
    if (errorMessage && typeof message === "string" && message.trim() !== "") {
      errorMessage.textContent = message;
    }
    errorView.classList.remove("hidden");
  }

  function formatSpeed(speed) {
    if (typeof speed !== "number" || !Number.isFinite(speed) || speed < 0)
      speed = 0;
    if (speed >= 1000)
      return { value: (speed / 1000).toFixed(2), unit: "Gbps" };
    return { value: speed.toFixed(1), unit: "Mbps" };
  }

  function safeFixed(v, digits) {
    return typeof v === "number" && Number.isFinite(v)
      ? v.toFixed(digits)
      : "-";
  }

  async function consumeErrorBody(res) {
    try {
      await res.text();
    } catch (err) {
      console.debug("results page: failed to read error response body", err);
    }
  }

  function resultErrorMessage(statusCode) {
    if (statusCode === 404) {
      return "Result not found or has expired.";
    }
    if (statusCode >= 500) {
      return "Server error while loading result.";
    }
    return "Unable to load result.";
  }

  async function loadResult(resultID, signal) {
    const res = await fetch("/api/v1/results/" + resultID, { signal });
    if (!res.ok) {
      await consumeErrorBody(res);
      throw userError(resultErrorMessage(res.status));
    }
    return res.json();
  }

  function setText(el, text) {
    if (el) {
      el.textContent = text;
    }
  }

  function formatLatencyValue(v) {
    return typeof v === "number" && v > 0 ? safeFixed(v, 1) + " ms" : "-";
  }

  function updateServerDetails(d, refs) {
    if (!d.server_name) {
      return;
    }
    if (refs.serverLabelEl && refs.serverItemEl && refs.serverValueEl) {
      refs.serverLabelEl.classList.remove("hidden");
      refs.serverItemEl.classList.remove("hidden");
      refs.serverValueEl.textContent =
        typeof d.server_name === "string"
          ? d.server_name
          : String(d.server_name);
    }
  }

  function updateCreatedAt(createdAt, testedAtEl) {
    if (!createdAt || !testedAtEl) {
      return;
    }
    try {
      const date = new Date(createdAt);
      if (Number.isFinite(date.getTime())) {
        testedAtEl.textContent = date.toLocaleString();
      }
    } catch (err) {
      console.debug("results page: failed to parse created_at", err);
    }
  }

  function renderResult(d) {
    if (!d || typeof d !== "object") {
      showError("Invalid result payload.");
      return;
    }
    try {
      const downloadEl = document.getElementById("downloadResult");
      const uploadEl = document.getElementById("uploadResult");
      const latencyEl = document.getElementById("latencyResult");
      const jitterEl = document.getElementById("jitterResult");
      const loadedLatencyEl = document.getElementById("loadedLatencyResult");
      const bufferbloatEl = document.getElementById("bufferbloatResult");
      const ipv4El = document.getElementById("networkIPv4");
      const ipv6El = document.getElementById("networkIPv6");
      const serverLabelEl = document.getElementById("serverLabel");
      const serverItemEl = document.getElementById("serverItem");
      const serverValueEl = document.getElementById("serverValue");
      const testedAtEl = document.getElementById("testedAt");

      const dl = formatSpeed(
        Number.isFinite(d.download_mbps) ? d.download_mbps : 0,
      );
      const ul = formatSpeed(
        Number.isFinite(d.upload_mbps) ? d.upload_mbps : 0,
      );

      setText(downloadEl, dl.value);
      setText(uploadEl, ul.value);

      const dlUnit = document.querySelector(".result-primary .result-unit");
      const ulUnit = document.querySelector(".result-secondary .result-unit");
      setText(dlUnit, dl.unit);
      setText(ulUnit, ul.unit);

      setText(latencyEl, formatLatencyValue(d.latency_ms));
      setText(jitterEl, formatLatencyValue(d.jitter_ms));
      setText(loadedLatencyEl, formatLatencyValue(d.loaded_latency_ms));
      if (bufferbloatEl) {
        bufferbloatEl.textContent =
          typeof d.bufferbloat_grade === "string" && d.bufferbloat_grade
            ? d.bufferbloat_grade
            : "-";
      }

      setText(ipv4El, d.ipv4 || "-");
      setText(ipv6El, d.ipv6 || "-");
      updateServerDetails(d, { serverLabelEl, serverItemEl, serverValueEl });
      updateCreatedAt(d.created_at, testedAtEl);

      document.title =
        "openByte — " +
        dl.value +
        " " +
        dl.unit +
        " / " +
        ul.value +
        " " +
        ul.unit;
    } catch (err) {
      console.error("results page: render failed", err);
      showError("Failed to render result.");
    }
  }
})();
