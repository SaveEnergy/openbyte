/** English source catalog. Keys are stable UI concepts, not source sentences. */

export const en = Object.freeze({
  "meta.shared.title": "openByte — Shared Result",

  "common.skipToMain": "Skip to main content",
  "common.error": "Error",
  "common.success": "Success",
  "language.label": "Choose language",
  "language.system": "System · {locale}",

  "theme.switch": "Switch theme",
  "theme.systemNextLight":
    "Theme follows system preference. Activate for light theme.",
  "theme.lightNextDark": "Light theme active. Activate for dark theme.",
  "theme.darkNextSystem":
    "Dark theme active. Activate to follow system preference.",

  "server.connecting": "Connecting…",
  "server.ready": "Ready",
  "server.offline": "Offline",
  "test.heading": "openByte internet speed test",
  "test.startAria": "GO — Start speed test",
  "test.startShort": "GO",
  "test.connecting": "Connecting…",
  "test.readyHint": "Start speed test",
  "test.offlineHint": "Offline — retrying…",
  "network.publicIp": "Public IP addresses",
  "network.detecting": "Detecting…",
  "network.notDetected": "Not available",
  "test.progressAria": "Network measurement in progress",
  "test.progressText": "Measuring network",
  "test.phasesAria": "Test phases",
  "test.phase.ping": "Ping",
  "test.phase.download": "Download",
  "test.phase.upload": "Upload",
  "test.phaseInProgress": "{phase} in progress",
  "test.cancel": "Cancel",

  "result.heading": "Result",
  "result.download": "Download",
  "result.upload": "Upload",
  "result.sharedHeading": "Test result",
  "result.server": "Server",
  "result.tested": "Tested at",
  "result.loading": "Loading result…",
  "result.loadedLatencyAdvisory":
    "Latency rises under load, so calls and games may lag while the connection is busy.",
  "metric.idleLatency": "Idle",
  "metric.jitter": "Jitter",
  "metric.loadedLatency": "Under load",
  "metric.bufferbloat": "Bufferbloat",
  "metric.helpSummary": "What do these numbers mean?",
  "metric.idleLatencyDescription":
    "Round-trip time while the connection is quiet. Lower is better.",
  "metric.jitterDescription":
    "Variation between latency measurements. High jitter can disrupt calls and games.",
  "metric.loadedLatencyDescription":
    "Round-trip time while downloading or uploading. Lower means the connection stays responsive under load.",
  "metric.bufferbloatDescription":
    "How much latency rises under load, graded from A+ to F. Lower grades mean more lag when the connection is busy.",
  "network.publicIpAtTest": "Public IP addresses for this test",
  "history.heading": "Recent tests",
  "history.justNow": "just now",

  "action.testAgain": "Test again",
  "action.share": "Share",
  "action.runOwnTest": "Run your own test",
  "action.runSpeedTest": "Run a speed test",
  "share.preparing": "Creating link…",
  "share.copied": "Link copied",
  "share.unavailable": "Unable to create share link right now",
  "share.nativeTitle": "openByte Speed Test Result",
  "share.copyPrompt": "Copy this link:",
  "nav.speedTest": "Speed Test",
  "nav.privacy": "Privacy",
  "nav.impressum": "Legal Notice",

  "privacy.meta.title": "openByte — Privacy",
  "privacy.heading": "Data privacy",
  "privacy.intro":
    "This page explains what data this openByte server processes when you use the speed test, and what stays on your device.",
  "privacy.test.heading": "Running a speed test",
  "privacy.test.body":
    "A speed test transfers randomly generated data between your browser and this server; the transferred data itself is meaningless and is discarded. While the test runs, the server uses your IP address in memory to enforce per-address transfer and request limits. Neither the test nor your IP address is written to its database, and no result is stored on the server unless you share it.",
  "privacy.ip.heading": "Public IP addresses",
  "privacy.ip.body":
    "The page shows the public IPv4 and IPv6 addresses your browser uses to reach this server. They are looked up when the page loads and are only displayed to you; the server does not store them.",
  "privacy.share.heading": "Shared results",
  "privacy.share.body":
    "A result is stored on this server only when you tap Share. The stored record contains the measured values (download, upload, latency, jitter, latency under load, and the bufferbloat grade), the public IP addresses shown with the result, the server name, and the time of the test. It is published under a random link: anyone who knows the link can view it. Shared results are deleted after 90 days at the latest, or earlier when the server trims its stored results.",
  "privacy.local.heading": "Data stored in your browser",
  "privacy.local.body":
    "Your language choice, your theme choice, and a short list of your recent test results are kept in your browser's local storage so the page can restore them on your next visit. This data never leaves your device; you can remove it at any time by clearing this site's browsing data.",
  "privacy.tracking.heading": "Cookies, tracking, and third parties",
  "privacy.tracking.body":
    "This site sets no cookies, uses no analytics, and loads nothing from third parties. Fonts and all other assets are served by this server.",
  "privacy.logs.heading": "Server logs",
  "privacy.logs.body":
    "To keep the service reliable and to protect it from abuse, the server may write technical log entries for API requests, including the request path, status code, duration, and IP address. How long logs are kept is decided by the operator of this instance.",
  "privacy.operator.heading": "Who is responsible",
  "privacy.operator.body":
    "openByte is self-hosted open-source software; this instance is run by its own operator, who is responsible for the deployment, including any infrastructure in front of it (such as a reverse proxy) that may process additional data. If this deployment provides a legal notice, you can reach it from the link in the footer.",

  "announcement.complete":
    "Speed test complete. Download {download}. Upload {upload}. Latency {latency}.",
  "announcement.completeWithGrade":
    "Speed test complete. Download {download}. Upload {upload}. Latency {latency}. Bufferbloat grade {grade}.",
  "error.testInProgress": "Test already in progress",
  "error.serverNotReady": "Server is not ready yet",
  "error.testFailed": "Speed test failed. Please try again.",
  "worker.unsupported": "This browser cannot run the speed test.",
  "worker.failed": "The speed test stopped. Please try again.",
  "worker.unreadable": "The speed test returned an invalid response.",
  "download.network":
    "Network error during download. Please try again.",
  "upload.network": "Network error during upload. Please try again.",
  "server.overloaded":
    "Server overloaded. Please try again in a moment.",
  "download.noStreams": "Download test failed. Please try again.",
  "upload.noStreams": "Upload test failed. Please try again.",
  "error.resultNotFound": "Result not found or has expired.",
  "error.resultServer": "Server error while loading result.",
  "error.resultUnavailable": "Unable to load result.",
  "error.resultInvalidPayload": "Invalid result payload.",
  "error.resultRender": "Failed to render result.",
  "error.resultInvalidId": "This result link is invalid.",
  "error.resultTimeout": "Loading the result timed out. Please try again.",
});
