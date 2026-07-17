/** English source catalog. Keys are stable UI concepts, not source sentences. */

export const en = Object.freeze({
  "meta.shared.title": "openByte — Shared Result",

  "common.skipToMain": "Skip to main content",
  "common.error": "Error",
  "common.success": "Success",
  "preferences.title": "Preferences",
  "preferences.language": "Language",
  "preferences.appearance": "Appearance",
  "language.system": "System · {locale}",
  "theme.system": "System",
  "theme.light": "Light",
  "theme.dark": "Dark",

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
  "history.remember": "Save recent results on this device",
  "history.rememberHint":
    "Stores up to 10 results in this browser. Turning this off deletes them.",

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
  "privacy.heading": "Privacy and data handling",
  "privacy.intro":
    "This technical summary explains what the openByte application itself processes and what stays on your device.",
  "privacy.updated": "Last updated: 17 July 2026",
  "privacy.scope.heading": "About this technical summary",
  "privacy.scope.body":
    "openByte is self-hosted software, so this built-in page is not a complete operator-specific notice under Article 13 GDPR. The operator must publish its own notice and can configure /privacy to redirect to it.",
  "privacy.operator.heading": "Operator-specific privacy notice",
  "privacy.operator.body":
    "A complete operator-specific privacy notice must identify the controller and contact details, any data protection officer, the purposes and legal bases, applicable legitimate interests, and the right to complain to a supervisory authority.",
  "privacy.test.heading": "Requests, speed tests, and IP addresses",
  "privacy.test.body":
    "Every request necessarily exposes its source IP address to this server and any reverse proxy. openByte uses it to answer the request, display the public IPv4 and IPv6 addresses, enforce transfer and request limits, and protect service capacity. The download and upload tests exchange meaningless random bytes that are discarded. A completed measurement is not written to the results database unless you choose Share.",
  "privacy.share.heading": "Sharing a result",
  "privacy.share.body":
    "Only activating Share sends a result to the server database. The record contains download, upload, latency, jitter, latency under load, bufferbloat grade, the displayed public IPv4 and IPv6 addresses, server name, and the time it was shared. The random result link has no access control: anyone who knows it can view the record.",
  "privacy.local.heading": "Storage on your device",
  "privacy.local.body":
    "Language and theme are stored in local storage only after you select them. Recent-test history is off by default; enabling it stores the setting and up to 10 results (measurements, time, and grade) in this browser until you turn it off or clear site data. The local copy is not transmitted automatically. Failed optional address probes are remembered only in page memory and are not written to device storage.",
  "privacy.tracking.heading": "Cookies and tracking",
  "privacy.tracking.body":
    "openByte sets no cookies, performs no analytics or advertising tracking, and embeds no third-party resources. Fonts and other application assets come from this server. Following an external link or an operator-configured redirect sends a request to that destination only after you navigate there.",
  "privacy.recipients.heading": "Logs, recipients, and transfers",
  "privacy.recipients.body":
    "API logs may contain method, path, status, duration, and IP address. The operator and its configured reverse-proxy, hosting, logging, storage, or support providers may receive request data; anyone with a shared-result link receives that record. openByte itself sends no data to analytics or ad networks. Only the operator can name its actual processors, international transfers, and safeguards in its complete notice.",
  "privacy.retention.heading": "Retention",
  "privacy.retention.body":
    "Transfer bytes are discarded with the request. IP rate-limit entries stay in volatile memory; inactive entries become eligible for cleanup after 10 minutes and disappear on later cleanup or a restart. An hourly job removes shared results once they are older than 90 days, and the configured count limit may remove them earlier. Operational failures can delay cleanup. The operator controls log and any backup retention and must disclose those periods in its notice.",
  "privacy.rights.heading": "Your choices and rights",
  "privacy.rights.body":
    "Where GDPR applies, you may have rights to access, rectification, erasure, restriction, data portability, and objection, and to complain to a supervisory authority. Exercise those rights against the operator named in its notice; because openByte has no accounts, a result link or request details may be needed to find data. Turn off recent-test history or clear site data to remove browser-stored results.",
  "privacy.required.heading": "Required data and automated decisions",
  "privacy.required.body":
    "An IP address is technically required for the server to answer; without a network request the page and speed test cannot be provided. Running a measurement and sharing it are optional, and openByte creates no statutory or contractual duty to provide data. It performs no profiling and makes no decision covered by Article 22 GDPR.",

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
