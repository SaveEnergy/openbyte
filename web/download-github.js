/** GitHub Releases API fetch for the download page. */

import { releaseUrl } from "./download-platform.js";

export async function fetchLatestRelease() {
  const res = await fetch(releaseUrl);
  if (!res.ok) {
    const reason =
      res.status === 403
        ? "GitHub API rate limited"
        : "GitHub API error " + res.status;
    try {
      await res.text();
    } catch (err) {
      console.debug("download page: failed to read release error body", err);
    }
    throw new Error(reason);
  }
  return res.json();
}
