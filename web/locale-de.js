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
  "history.remember": "Letzte Tests merken",
  "history.rememberHint":
    "Speichert bis zu 10 Ergebnisse in diesem Browser, bis Sie die Funktion ausschalten.",

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
  "privacy.heading": "Datenschutz und Datenverarbeitung",
  "privacy.intro":
    "Diese technische Zusammenfassung erklärt, welche Daten die openByte-Anwendung selbst verarbeitet und welche Daten auf Ihrem Gerät bleiben.",
  "privacy.updated": "Stand: 17. Juli 2026",
  "privacy.scope.heading": "Zu dieser technischen Zusammenfassung",
  "privacy.scope.body":
    "openByte ist selbst gehostete Software. Diese integrierte Seite ist daher keine vollständige, betreiberspezifische Information nach Art. 13 DSGVO. Der Betreiber muss eigene Datenschutzhinweise veröffentlichen und kann /privacy dorthin weiterleiten.",
  "privacy.operator.heading": "Verantwortlicher und vollständige Hinweise",
  "privacy.operator.body":
    "Verantwortlicher ist die Person oder Organisation, die diese Instanz betreibt – nicht das openByte-Projekt oder seine Mitwirkenden. Die Hinweise des Betreibers müssen Namen und Kontaktdaten des Verantwortlichen, gegebenenfalls den Datenschutzbeauftragten, Zwecke und Rechtsgrundlagen, gegebenenfalls berechtigte Interessen sowie das Beschwerderecht bei einer Aufsichtsbehörde nennen.",
  "privacy.operator.link": "Impressum des Betreibers öffnen",
  "privacy.test.heading": "Anfragen, Speedtests und IP-Adressen",
  "privacy.test.body":
    "Bei jeder Anfrage wird die Quell-IP-Adresse zwangsläufig von diesem Server und einem etwaigen Reverse-Proxy verarbeitet. openByte verwendet sie, um zu antworten, die öffentliche IPv4- und IPv6-Adresse anzuzeigen, Übertragungs- und Anfragelimits durchzusetzen und die Dienstkapazität zu schützen. Die Download- und Uploadtests tauschen bedeutungslose Zufallsdaten aus, die verworfen werden. Ein abgeschlossenes Messergebnis wird nur dann in die Ergebnisdatenbank geschrieben, wenn Sie Teilen wählen.",
  "privacy.share.heading": "Ergebnis teilen",
  "privacy.share.body":
    "Erst das Aktivieren von Teilen sendet ein Ergebnis an die Serverdatenbank. Der Eintrag enthält Download, Upload, Latenz, Jitter, Latenz unter Last, Bufferbloat-Bewertung, die angezeigten öffentlichen IPv4- und IPv6-Adressen, den Servernamen und den Zeitpunkt des Teilens. Der zufällige Ergebnislink hat keine Zugriffskontrolle: Wer ihn kennt, kann den Eintrag ansehen.",
  "privacy.local.heading": "Speicherung auf Ihrem Gerät",
  "privacy.local.body":
    "Sprache und Farbschema werden erst nach Ihrer Auswahl im lokalen Speicher abgelegt. Der Verlauf letzter Tests ist standardmäßig ausgeschaltet; beim Einschalten speichert der Browser die Einstellung und bis zu 10 Ergebnisse (Messwerte, Zeitpunkt und Bewertung), bis Sie die Funktion ausschalten oder Websitedaten löschen. Die lokale Kopie wird nicht automatisch übertragen. Fehlgeschlagene optionale Adressabfragen werden für diesen Tab höchstens 24 Stunden im Sitzungsspeicher vermerkt.",
  "privacy.tracking.heading": "Cookies und Tracking",
  "privacy.tracking.body":
    "openByte setzt keine Cookies, führt kein Analyse- oder Werbetracking durch und bindet keine Ressourcen Dritter ein. Schriften und andere Anwendungsdateien kommen von diesem Server. Erst wenn Sie einem externen Link oder einer vom Betreiber konfigurierten Weiterleitung folgen, wird eine Anfrage an dieses Ziel gesendet.",
  "privacy.recipients.heading": "Protokolle, Empfänger und Übermittlungen",
  "privacy.recipients.body":
    "API-Protokolle können Methode, Pfad, Status, Dauer und IP-Adresse enthalten. Der Betreiber und seine eingesetzten Reverse-Proxy-, Hosting-, Protokollierungs-, Speicher- oder Supportanbieter können Anfragedaten erhalten; wer einen geteilten Ergebnislink kennt, erhält diesen Eintrag. openByte selbst sendet keine Daten an Analyse- oder Werbenetzwerke. Nur der Betreiber kann in seinen vollständigen Hinweisen die tatsächlichen Auftragsverarbeiter, Drittlandübermittlungen und Garantien nennen.",
  "privacy.retention.heading": "Speicherdauer",
  "privacy.retention.body":
    "Übertragene Testdaten werden mit der Anfrage verworfen. IP-Einträge für Anfragelimits bleiben im flüchtigen Arbeitsspeicher; inaktive Einträge können nach 10 Minuten bereinigt werden und verschwinden bei einer späteren Bereinigung oder einem Neustart. Ein stündlicher Vorgang entfernt geteilte Ergebnisse, sobald sie älter als 90 Tage sind; die konfigurierte Höchstzahl kann sie früher entfernen. Betriebsstörungen können die Bereinigung verzögern. Der Betreiber bestimmt die Aufbewahrung von Protokollen und etwaigen Sicherungen und muss diese Fristen in seinen Hinweisen nennen.",
  "privacy.rights.heading": "Ihre Wahlmöglichkeiten und Rechte",
  "privacy.rights.body":
    "Soweit die DSGVO gilt, können Ihnen Rechte auf Auskunft, Berichtigung, Löschung, Einschränkung, Datenübertragbarkeit und Widerspruch sowie ein Beschwerderecht bei einer Datenschutzaufsichtsbehörde zustehen. Richten Sie diese Rechte an den Betreiber aus seinen Hinweisen; weil openByte keine Konten führt, können ein Ergebnislink oder Anfragedetails zur Zuordnung nötig sein. Schalten Sie den Verlauf aus oder löschen Sie Websitedaten, um im Browser gespeicherte Ergebnisse zu entfernen.",
  "privacy.required.heading": "Erforderliche Daten und automatisierte Entscheidungen",
  "privacy.required.body":
    "Eine IP-Adresse ist technisch erforderlich, damit der Server antworten kann; ohne Netzwerkanfrage können Seite und Speedtest nicht bereitgestellt werden. Eine Messung durchzuführen und sie zu teilen ist freiwillig, und openByte begründet keine gesetzliche oder vertragliche Pflicht zur Datenbereitstellung. Es erstellt keine Profile und trifft keine Entscheidung im Sinne des Art. 22 DSGVO.",

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
