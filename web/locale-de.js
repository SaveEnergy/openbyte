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
  "result.download": "Download",
  "result.upload": "Upload",
  "result.sharedHeading": "Testergebnis",
  "result.server": "Server",
  "result.tested": "Getestet am",
  "result.loading": "Ergebnis wird geladen…",
  "result.loadedLatencyAdvisory":
    "Die Latenz steigt unter Last. Anrufe und Spiele können dann stocken.",
  "metric.idleLatency": "Leerlauf",
  "metric.jitter": "Jitter",
  "metric.loadedLatency": "Unter Last",
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
  "nav.privacy": "Datenschutz",
  "nav.impressum": "Impressum",

  "privacy.meta.title": "openByte — Datenschutz",
  "privacy.heading": "Datenschutz",
  "privacy.intro":
    "Diese Seite erklärt, welche Daten dieser openByte-Server bei der Nutzung des Speedtests verarbeitet und welche Daten auf Ihrem Gerät bleiben.",
  "privacy.test.heading": "Während eines Speedtests",
  "privacy.test.body":
    "Ein Speedtest überträgt zufällig erzeugte Daten zwischen Ihrem Browser und diesem Server; die übertragenen Daten selbst sind bedeutungslos und werden verworfen. Während des Tests verwendet der Server Ihre IP-Adresse im Arbeitsspeicher, um Übertragungs- und Anfragelimits pro Adresse durchzusetzen. Weder der Test noch Ihre IP-Adresse werden in der Datenbank gespeichert; ein Ergebnis wird nur dann auf dem Server abgelegt, wenn Sie es teilen.",
  "privacy.ip.heading": "Öffentliche IP-Adressen",
  "privacy.ip.body":
    "Die Seite zeigt die öffentlichen IPv4- und IPv6-Adressen, mit denen Ihr Browser diesen Server erreicht. Sie werden beim Laden der Seite ermittelt und nur Ihnen angezeigt; der Server speichert sie nicht.",
  "privacy.share.heading": "Geteilte Ergebnisse",
  "privacy.share.body":
    "Ein Ergebnis wird nur dann auf diesem Server gespeichert, wenn Sie auf „Teilen“ tippen. Der gespeicherte Eintrag enthält die Messwerte (Download, Upload, Latenz, Jitter, Latenz unter Last und die Bufferbloat-Bewertung), die zum Ergebnis angezeigten öffentlichen IP-Adressen, den Servernamen und den Testzeitpunkt. Er ist unter einem zufälligen Link erreichbar: Wer den Link kennt, kann das Ergebnis sehen. Geteilte Ergebnisse werden spätestens nach 90 Tagen gelöscht, bei Platzmangel auch früher.",
  "privacy.local.heading": "Daten in Ihrem Browser",
  "privacy.local.body":
    "Ihre Sprachwahl, Ihr Farbschema und eine kurze Liste Ihrer letzten Testergebnisse liegen im lokalen Speicher Ihres Browsers, damit die Seite sie beim nächsten Besuch wiederherstellen kann. Diese Daten verlassen Ihr Gerät nicht; Sie können sie jederzeit über die Websitedaten-Einstellungen Ihres Browsers löschen.",
  "privacy.tracking.heading": "Cookies, Tracking und Dritte",
  "privacy.tracking.body":
    "Diese Seite setzt keine Cookies, verwendet keine Analysedienste und lädt nichts von Dritten. Schriften und alle weiteren Inhalte liefert dieser Server selbst aus.",
  "privacy.logs.heading": "Server-Protokolle",
  "privacy.logs.body":
    "Um den Dienst zuverlässig zu betreiben und vor Missbrauch zu schützen, kann der Server technische Protokolleinträge zu API-Anfragen schreiben, einschließlich Pfad, Statuscode, Dauer und IP-Adresse. Wie lange Protokolle aufbewahrt werden, entscheidet der Betreiber dieser Instanz.",
  "privacy.operator.heading": "Verantwortlichkeit",
  "privacy.operator.body":
    "openByte ist selbst gehostete Open-Source-Software; diese Instanz wird von ihrem eigenen Betreiber betrieben. Er ist für den Betrieb verantwortlich, einschließlich vorgelagerter Infrastruktur (etwa eines Reverse-Proxys), die zusätzliche Daten verarbeiten kann. Stellt diese Installation ein Impressum bereit, erreichen Sie es über den Link in der Fußzeile.",

  "announcement.complete":
    "Speedtest abgeschlossen. Download {download}. Upload {upload}. Latenz {latency}.",
  "announcement.completeWithGrade":
    "Speedtest abgeschlossen. Download {download}. Upload {upload}. Latenz {latency}. Bufferbloat-Bewertung {grade}.",
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
