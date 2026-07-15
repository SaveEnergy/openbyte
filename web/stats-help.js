/** Shared metric explanations for the "What do these numbers mean?" panel. */

const METRIC_EXPLANATIONS = [
  [
    "Idle Latency",
    "Round-trip time to the server while the connection is quiet. " +
      "Lower is better; under 20 ms feels instant.",
  ],
  [
    "Jitter",
    "How much latency varies between pings. High jitter causes choppy " +
      "calls and unstable game connections.",
  ],
  [
    "Loaded Latency",
    "Latency measured while the connection is fully busy downloading or " +
      "uploading — what you feel during heavy use.",
  ],
  [
    "Bufferbloat",
    "Grade for how much latency rises under load (A+ best, F worst). " +
      "Poor grades mean lag in calls and games while others use the " +
      "connection.",
  ],
];

const list = document.getElementById("statsHelpList");
if (list && list.childElementCount === 0) {
  for (const [term, explanation] of METRIC_EXPLANATIONS) {
    const dt = document.createElement("dt");
    dt.textContent = term;
    const dd = document.createElement("dd");
    dd.textContent = explanation;
    list.append(dt, dd);
  }
}
