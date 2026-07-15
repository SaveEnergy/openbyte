/** German catalog. Technical units, protocol names, and product names stay invariant. */

export const de = Object.freeze({
  "meta.shared.title": "openByte — Geteiltes Testergebnis",

  "common.skipToMain": "Zum Hauptinhalt springen",
  "common.error": "Fehler",
  "common.success": "Erfolgreich",
  "language.label": "Sprache auswählen",
  "language.system": "System · {locale}",

  "theme.switch": "Farbschema wechseln",
  "theme.systemNextLight":
    "Das Farbschema folgt der Systemeinstellung. Aktivieren, um das helle Farbschema zu verwenden.",
  "theme.lightNextDark":
    "Helles Farbschema aktiv. Aktivieren, um zum dunklen Farbschema zu wechseln.",
  "theme.darkNextSystem":
    "Dunkles Farbschema aktiv. Aktivieren, um der Systemeinstellung zu folgen.",

  "server.connecting": "Verbindung wird hergestellt…",
  "server.ready": "Bereit",
  "server.offline": "Offline",
  "test.heading": "Internet-Speedtest mit openByte",
  "test.startAria": "GO — Speedtest starten",
  "test.startShort": "GO",
  "test.connecting": "Wird verbunden…",
  "test.readyHint": "Speedtest starten",
  "test.offlineHint": "Offline – neuer Versuch…",
  "network.publicIp": "Öffentliche IP-Adressen",
  "network.detecting": "Wird ermittelt…",
  "network.notDetected": "Nicht verfügbar",
  "test.progressAria": "Netzwerkmessung läuft",
  "test.progressText": "Netzwerk wird gemessen",
  "test.phasesAria": "Testphasen",
  "test.phase.ping": "Ping",
  "test.phase.download": "Download",
  "test.phase.upload": "Upload",
  "test.phaseInProgress": "{phase} läuft",
  "test.cancel": "Abbrechen",

  "result.heading": "Ergebnis",
  "result.partialNotice": "Teilergebnis – Upload nicht gemessen.",
  "result.download": "Download",
  "result.upload": "Upload",
  "result.notMeasured": "nicht gemessen",
  "result.sharedHeading": "Testergebnis",
  "result.server": "Server",
  "result.tested": "Getestet am",
  "result.loading": "Ergebnis wird geladen…",
  "result.loadedLatencyAdvisory":
    "Die Latenz steigt unter Last. Anrufe und Spiele können dann stocken.",
  "metric.idleLatency": "Leerlauf",
  "metric.jitter": "Jitter",
  "metric.loadedLatency": "Unter Last",
  "metric.downloadLatency": "Beim Download",
  "metric.bufferbloat": "Bufferbloat",
  "metric.helpSummary": "Was bedeuten die Werte?",
  "metric.idleLatencyDescription":
    "Zeit zum Server und zurück, wenn die Verbindung frei ist. Niedriger ist besser.",
  "metric.jitterDescription":
    "Schwankung zwischen Latenzmessungen. Hoher Jitter stört Anrufe und Spiele.",
  "metric.loadedLatencyDescription":
    "Zeit zum Server und zurück, während die Verbindung ausgelastet ist. Je niedriger, desto reaktionsschneller bleibt sie.",
  "metric.bufferbloatDescription":
    "Anstieg der Latenz unter Last, bewertet von A+ bis F. Niedrige Noten bedeuten mehr Verzögerung.",
  "network.publicIpAtTest": "Öffentliche IP-Adressen beim Test",
  "history.heading": "Letzte Tests",
  "history.justNow": "gerade eben",

  "action.testAgain": "Nochmal testen",
  "action.share": "Teilen",
  "action.runOwnTest": "Eigenen Speedtest starten",
  "action.runSpeedTest": "Speedtest starten",
  "share.preparing": "Link wird erstellt…",
  "share.copied": "Link kopiert",
  "share.unavailable":
    "Der Link zum Teilen kann gerade nicht erstellt werden",
  "share.nativeTitle": "openByte-Speedtest-Ergebnis",
  "share.copyPrompt": "Diesen Link kopieren:",
  "nav.speedTest": "Speedtest",

  "announcement.complete":
    "Speedtest abgeschlossen. Download {download}. Upload {upload}. Latenz {latency}.",
  "announcement.completeWithGrade":
    "Speedtest abgeschlossen. Download {download}. Upload {upload}. Latenz {latency}. Bufferbloat-Bewertung {grade}.",
  "announcement.partial":
    "Speedtest vorzeitig abgebrochen. Download {download}. Latenz {latency}.",
  "error.testInProgress": "Ein Test läuft bereits.",
  "error.serverNotReady": "Der Server ist noch nicht bereit.",
  "error.testFailed": "Speedtest fehlgeschlagen. Bitte erneut versuchen.",
  "worker.unsupported": "Dieser Browser kann den Speedtest nicht ausführen.",
  "worker.failed": "Der Speedtest wurde gestoppt. Bitte erneut versuchen.",
  "worker.unreadable": "Der Speedtest hat ungültige Daten geliefert.",
  "download.network":
    "Netzwerkfehler während des Downloads. Bitte erneut versuchen.",
  "upload.network":
    "Netzwerkfehler während des Uploads. Bitte erneut versuchen.",
  "server.overloaded":
    "Der Server ist überlastet. Bitte in Kürze erneut versuchen.",
  "download.noStreams": "Download-Test fehlgeschlagen. Bitte erneut versuchen.",
  "upload.noStreams": "Upload-Test fehlgeschlagen. Bitte erneut versuchen.",
  "error.resultNotFound":
    "Ergebnis nicht gefunden oder nicht mehr verfügbar.",
  "error.resultServer": "Serverfehler beim Laden des Ergebnisses.",
  "error.resultUnavailable": "Ergebnis kann nicht geladen werden.",
  "error.resultInvalidPayload": "Ungültige Ergebnisdaten.",
  "error.resultRender": "Ergebnis kann nicht angezeigt werden.",
  "error.resultInvalidId": "Dieser Ergebnislink ist ungültig.",
  "error.resultTimeout":
    "Das Laden des Ergebnisses dauert zu lange. Bitte erneut versuchen.",
});
