/** English source catalog. Keys are stable UI concepts, not source sentences. */

export const en = Object.freeze({
  "meta.home.description":
    "Open-source internet speed test. Measure download, upload, latency, jitter, and bufferbloat.",
  "meta.home.ogTitle": "openByte — Speed Test",
  "meta.shared.description":
    "openByte speed test result — download, upload, latency, and bufferbloat.",
  "meta.shared.ogTitle": "openByte — Speed Test Result",
  "meta.shared.ogDescription":
    "See this internet speed test result on openByte.",
  "meta.shared.title": "openByte — Shared Result",

  "common.skipToMain": "Skip to main content",
  "common.error": "Error",
  "common.success": "Success",
  "language.label": "Choose language",
  "language.auto": "Auto",

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
  "test.readyHint": "Click to test your speed",
  "test.offlineHint": "Server offline — retrying",
  "network.publicIp": "Your public IP",
  "network.detecting": "Detecting…",
  "network.notDetected": "Not detected",
  "test.progressAria": "Network measurement in progress",
  "test.progressText": "Measuring network",
  "test.phasesAria": "Test phases",
  "test.phase.ping": "Ping",
  "test.phase.download": "Download",
  "test.phase.upload": "Upload",
  "test.stage.saturating": "Saturating",
  "test.stage.measuring": "Measuring",
  "test.phaseInProgress": "{phase} in progress",
  "test.cancel": "Cancel",

  "result.heading": "Results",
  "result.partialNotice":
    "Test cancelled early — upload was not measured.",
  "result.download": "Download",
  "result.upload": "Upload",
  "result.notMeasured": "not measured",
  "result.sharedHeading": "Shared Result",
  "result.server": "Server",
  "result.tested": "Tested",
  "result.loading": "Loading result…",
  "metric.idleLatency": "Idle Latency",
  "metric.jitter": "Jitter",
  "metric.loadedLatency": "Loaded Latency",
  "metric.bufferbloat": "Bufferbloat",
  "metric.helpSummary": "What do these numbers mean?",
  "metric.idleLatencyDescription":
    "Round-trip time to the server while the connection is quiet. Lower is better; under 20 ms feels instant.",
  "metric.jitterDescription":
    "How much latency varies between pings. High jitter causes choppy calls and unstable game connections.",
  "metric.loadedLatencyDescription":
    "Latency measured while the connection is fully busy downloading or uploading — what you feel during heavy use.",
  "metric.bufferbloatDescription":
    "Grade for how much latency rises under load (A+ best, F worst). Poor grades mean lag in calls and games while others use the connection.",
  "network.publicIpAtTest": "Public IP at test time",
  "history.heading": "Recent Tests on This Device",
  "history.justNow": "just now",

  "action.testAgain": "Test Again",
  "action.share": "Share",
  "action.runOwnTest": "Run Your Own Test",
  "action.runSpeedTest": "Run a Speed Test",
  "share.preparing": "Preparing…",
  "share.copied": "Link copied to clipboard",
  "share.unavailable": "Unable to create share link right now",
  "share.nativeTitle": "openByte Speed Test Result",
  "share.copyPrompt": "Copy this link:",
  "nav.speedTest": "Speed Test",

  "verdict.partial.exceptional":
    "Exceptional download speed — ample for multiple 4K streams and large downloads.",
  "verdict.partial.excellent":
    "Excellent download speed — smooth 4K streaming and fast downloads.",
  "verdict.partial.good":
    "Good download speed — comfortable HD streaming and browsing.",
  "verdict.partial.modest":
    "Modest download speed — fine for browsing and music.",
  "verdict.partial.slow":
    "Slow download speed — expect buffering and long downloads.",
  "verdict.complete.exceptional":
    "Exceptional connection — handles 4K streaming, cloud backups, and busy households with ease.",
  "verdict.complete.excellent":
    "Excellent connection — smooth 4K streaming, video calls, and gaming.",
  "verdict.complete.good":
    "Good connection — comfortable HD streaming and stable video calls.",
  "verdict.complete.modest":
    "Modest connection — fine for browsing and music; large downloads take a while.",
  "verdict.complete.slow":
    "Slow connection — expect buffering and long download times.",
  "verdict.bufferbloatWarning":
    "Latency rises noticeably under load (bufferbloat), which can cause lag in calls and games while the connection is busy.",

  "announcement.complete":
    "Speed test complete. Download {download}. Upload {upload}. Latency {latency}.",
  "announcement.completeWithGrade":
    "Speed test complete. Download {download}. Upload {upload}. Latency {latency}. Bufferbloat grade {grade}.",
  "announcement.partial":
    "Speed test cancelled early. Download {download}. Latency {latency}.",
  "announcement.partialWithGrade":
    "Speed test cancelled early. Download {download}. Latency {latency}. Bufferbloat grade {grade}.",

  "error.testInProgress": "Test already in progress",
  "error.serverNotReady": "Server is not ready yet",
  "error.testFailed": "Speed test failed. Please try again.",
  "error.workerUnsupported": "This browser does not support Web Workers.",
  "error.workerFailed": "The speed test worker failed. Please try again.",
  "error.workerUnreadable":
    "The speed test worker returned an unreadable response.",
  "error.downloadNetwork":
    "Network error during download. Please try again.",
  "error.uploadNetwork": "Network error during upload. Please try again.",
  "error.serverOverloaded":
    "Server overloaded. Please try again in a moment.",
  "error.downloadNoStreams":
    "Download failed. No stream completed successfully.",
  "error.uploadNoStreams":
    "Upload failed. No stream completed successfully.",
  "error.resultNotFound": "Result not found or has expired.",
  "error.resultServer": "Server error while loading result.",
  "error.resultUnavailable": "Unable to load result.",
  "error.resultInvalidPayload": "Invalid result payload.",
  "error.resultRender": "Failed to render result.",
  "error.resultInvalidId": "Result ID format is invalid.",
  "error.resultTimeout": "Request timed out. Please try again.",
});
