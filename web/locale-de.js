/** German catalog. Technical units, protocol names, and product names stay invariant. */

export const de = Object.freeze({
  "meta.home.description":
    "Open-Source-Internet-Speedtest zum Messen von Download- und Upload-Geschwindigkeit, Latenz, Jitter und Bufferbloat.",
  "meta.home.ogTitle": "openByte — Speedtest",
  "meta.shared.description":
    "openByte-Speedtest-Ergebnis mit Download- und Upload-Geschwindigkeit, Latenz und Bufferbloat.",
  "meta.shared.ogTitle": "openByte — Speedtest-Ergebnis",
  "meta.shared.ogDescription":
    "Dieses mit openByte gemessene Speedtest-Ergebnis ansehen.",
  "meta.shared.title": "openByte — Geteiltes Testergebnis",

  "common.skipToMain": "Zum Hauptinhalt springen",
  "common.error": "Fehler",
  "common.success": "Erfolgreich",
  "language.label": "Sprache auswählen",
  "language.auto": "Auto",

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
  "test.heading": "openByte-Internet-Speedtest",
  "test.startAria": "LOS — Speedtest starten",
  "test.startShort": "LOS",
  "test.connecting": "Verbindung wird hergestellt…",
  "test.readyHint": "Zum Starten klicken",
  "test.offlineHint": "Server offline – erneuter Versuch folgt",
  "network.publicIp": "Öffentliche IP-Adresse",
  "network.detecting": "Wird ermittelt…",
  "network.notDetected": "Nicht erkannt",
  "test.progressAria": "Netzwerkmessung läuft",
  "test.progressText": "Netzwerk wird gemessen",
  "test.phasesAria": "Testphasen",
  "test.phase.ping": "Ping",
  "test.phase.download": "Download",
  "test.phase.upload": "Upload",
  "test.stage.saturating": "Auslastung",
  "test.stage.measuring": "Messung",
  "test.phaseInProgress": "{phase} läuft",
  "test.cancel": "Abbrechen",

  "result.heading": "Ergebnisse",
  "result.partialNotice":
    "Test vorzeitig abgebrochen – Upload wurde nicht gemessen.",
  "result.download": "Download",
  "result.upload": "Upload",
  "result.notMeasured": "nicht gemessen",
  "result.sharedHeading": "Geteiltes Testergebnis",
  "result.server": "Server",
  "result.tested": "Getestet",
  "result.loading": "Ergebnis wird geladen…",
  "metric.idleLatency": "Latenz im Leerlauf",
  "metric.jitter": "Jitter",
  "metric.loadedLatency": "Latenz unter Last",
  "metric.bufferbloat": "Bufferbloat",
  "metric.helpSummary": "Was bedeuten diese Werte?",
  "metric.idleLatencyDescription":
    "Zeit für ein Datenpaket zum Server und zurück, solange die Verbindung nicht ausgelastet ist. Je niedriger, desto besser; unter 20 ms sind Verzögerungen kaum spürbar.",
  "metric.jitterDescription":
    "Wie stark die Latenz zwischen einzelnen Pings schwankt. Hoher Jitter kann Anrufe abgehackt und Online-Spiele instabil machen.",
  "metric.loadedLatencyDescription":
    "Die Latenz, während die Verbindung durch Downloads oder Uploads voll ausgelastet ist – dieser Wert ist bei intensiver Nutzung spürbar.",
  "metric.bufferbloatDescription":
    "Bewertung dafür, wie stark die Latenz unter Last ansteigt (A+ ist am besten, F am schlechtesten). Schlechte Bewertungen bedeuten Verzögerungen bei Anrufen und Spielen, wenn andere die Verbindung nutzen.",
  "network.publicIpAtTest": "Öffentliche IP-Adresse zum Testzeitpunkt",
  "history.heading": "Letzte Tests auf diesem Gerät",
  "history.justNow": "gerade eben",

  "action.testAgain": "Erneut testen",
  "action.share": "Teilen",
  "action.runOwnTest": "Eigenen Speedtest starten",
  "action.runSpeedTest": "Speedtest starten",
  "share.preparing": "Wird vorbereitet…",
  "share.copied": "Link in die Zwischenablage kopiert",
  "share.unavailable":
    "Der Link zum Teilen kann gerade nicht erstellt werden",
  "share.nativeTitle": "openByte-Speedtest-Ergebnis",
  "share.copyPrompt": "Diesen Link kopieren:",
  "nav.speedTest": "Speedtest",

  "verdict.partial.exceptional":
    "Außergewöhnlich hohe Downloadgeschwindigkeit – mehr als genug für mehrere 4K-Streams und große Downloads.",
  "verdict.partial.excellent":
    "Sehr hohe Downloadgeschwindigkeit – flüssiges 4K-Streaming und schnelle Downloads.",
  "verdict.partial.good":
    "Gute Downloadgeschwindigkeit – problemloses HD-Streaming und Surfen.",
  "verdict.partial.modest":
    "Mäßige Downloadgeschwindigkeit – ausreichend zum Surfen und Musikhören.",
  "verdict.partial.slow":
    "Langsame Downloadgeschwindigkeit – mit Unterbrechungen und langen Downloadzeiten ist zu rechnen.",
  "verdict.complete.exceptional":
    "Außergewöhnlich schnelle Verbindung – bewältigt 4K-Streaming, Cloud-Backups und viele gleichzeitige Nutzer mühelos.",
  "verdict.complete.excellent":
    "Sehr schnelle Verbindung – flüssiges 4K-Streaming, Videoanrufe und Online-Gaming sind problemlos möglich.",
  "verdict.complete.good":
    "Gute Verbindung – problemloses HD-Streaming und stabile Videoanrufe.",
  "verdict.complete.modest":
    "Mäßig schnelle Verbindung – ausreichend zum Surfen und Musikhören; große Downloads dauern etwas länger.",
  "verdict.complete.slow":
    "Langsame Verbindung – mit Unterbrechungen und langen Downloadzeiten ist zu rechnen.",
  "verdict.bufferbloatWarning":
    "Die Latenz steigt unter Last deutlich an (Bufferbloat). Dadurch kann es bei Anrufen und Spielen zu Verzögerungen kommen, wenn die Verbindung ausgelastet ist.",

  "announcement.complete":
    "Speedtest abgeschlossen. Download {download}. Upload {upload}. Latenz {latency}.",
  "announcement.completeWithGrade":
    "Speedtest abgeschlossen. Download {download}. Upload {upload}. Latenz {latency}. Bufferbloat-Bewertung {grade}.",
  "announcement.partial":
    "Speedtest vorzeitig abgebrochen. Download {download}. Latenz {latency}.",
  "announcement.partialWithGrade":
    "Speedtest vorzeitig abgebrochen. Download {download}. Latenz {latency}. Bufferbloat-Bewertung {grade}.",

  "error.testInProgress": "Ein Test läuft bereits.",
  "error.serverNotReady": "Der Server ist noch nicht bereit.",
  "error.testFailed": "Speedtest fehlgeschlagen. Bitte erneut versuchen.",
  "error.workerUnsupported":
    "Dieser Browser unterstützt keine Web Worker.",
  "error.workerFailed":
    "Der Speedtest-Worker ist fehlgeschlagen. Bitte erneut versuchen.",
  "error.workerUnreadable":
    "Der Speedtest-Worker hat eine unlesbare Antwort zurückgegeben.",
  "error.downloadNetwork":
    "Netzwerkfehler während des Downloads. Bitte erneut versuchen.",
  "error.uploadNetwork":
    "Netzwerkfehler während des Uploads. Bitte erneut versuchen.",
  "error.serverOverloaded":
    "Der Server ist überlastet. Bitte in Kürze erneut versuchen.",
  "error.downloadNoStreams":
    "Download fehlgeschlagen. Kein Datenstrom wurde erfolgreich abgeschlossen.",
  "error.uploadNoStreams":
    "Upload fehlgeschlagen. Kein Datenstrom wurde erfolgreich abgeschlossen.",
  "error.resultNotFound":
    "Ergebnis nicht gefunden oder nicht mehr verfügbar.",
  "error.resultServer": "Serverfehler beim Laden des Ergebnisses.",
  "error.resultUnavailable": "Ergebnis kann nicht geladen werden.",
  "error.resultInvalidPayload": "Ungültige Ergebnisdaten.",
  "error.resultRender": "Ergebnis kann nicht angezeigt werden.",
  "error.resultInvalidId": "Ungültige Ergebnis-ID.",
  "error.resultTimeout":
    "Die Anfrage hat zu lange gedauert. Bitte erneut versuchen.",
});
