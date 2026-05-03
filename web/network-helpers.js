/** String helpers used by network probes. */

export function startsWithDigit(value) {
  if (!value || typeof value !== "string") return false;
  const code = value.codePointAt(0);
  if (typeof code !== "number") return false;
  return code >= 48 && code <= 57;
}
