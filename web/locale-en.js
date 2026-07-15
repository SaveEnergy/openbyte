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
