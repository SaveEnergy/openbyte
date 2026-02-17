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

  fetch("/api/v1/results/" + id, { signal: timeoutController.signal })
    .then(function (res) {
      if (!res.ok) {
        let message = "Unable to load result.";
        if (res.status === 404) {
          message = "Result not found or has expired.";
        } else if (res.status >= 500) {
          message = "Server error while loading result.";
        }
        return res
          .text()
          .catch(function (err) {
            console.debug(
              "results page: failed to read error response body",
              err,
            );
          })
          .then(function () {
            throw userError(message);
          });
      }
      return res.json();
    })
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
        typeof d.download_mbps === "number" && Number.isFinite(d.download_mbps)
          ? d.download_mbps
          : 0,
      );
      const ul = formatSpeed(
        typeof d.upload_mbps === "number" && Number.isFinite(d.upload_mbps)
          ? d.upload_mbps
          : 0,
      );

      if (downloadEl) downloadEl.textContent = dl.value;
      if (uploadEl) uploadEl.textContent = ul.value;

      const dlUnit = document.querySelector(".result-primary .result-unit");
      const ulUnit = document.querySelector(".result-secondary .result-unit");
      if (dlUnit) dlUnit.textContent = dl.unit;
      if (ulUnit) ulUnit.textContent = ul.unit;

      if (latencyEl)
        latencyEl.textContent =
          typeof d.latency_ms === "number" && d.latency_ms > 0
            ? safeFixed(d.latency_ms, 1) + " ms"
            : "-";
      if (jitterEl)
        jitterEl.textContent =
          typeof d.jitter_ms === "number" && d.jitter_ms > 0
            ? safeFixed(d.jitter_ms, 1) + " ms"
            : "-";
      if (loadedLatencyEl)
        loadedLatencyEl.textContent =
          typeof d.loaded_latency_ms === "number" && d.loaded_latency_ms > 0
            ? safeFixed(d.loaded_latency_ms, 1) + " ms"
            : "-";
      if (bufferbloatEl) {
        bufferbloatEl.textContent =
          typeof d.bufferbloat_grade === "string" && d.bufferbloat_grade
            ? d.bufferbloat_grade
            : "-";
      }

      if (ipv4El) ipv4El.textContent = d.ipv4 || "-";
      if (ipv6El) ipv6El.textContent = d.ipv6 || "-";

      if (d.server_name && serverLabelEl && serverItemEl && serverValueEl) {
        serverLabelEl.style.display = "";
        serverItemEl.style.display = "";
        serverValueEl.textContent =
          typeof d.server_name === "string"
            ? d.server_name
            : String(d.server_name);
      }

      if (d.created_at && testedAtEl) {
        try {
          const date = new Date(d.created_at);
          if (Number.isFinite(date.getTime())) {
            testedAtEl.textContent = date.toLocaleString();
          }
        } catch (err) {
          console.debug("results page: failed to parse created_at", err);
        }
      }

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
