/** Small dependency-free locale runtime for the browser presentation layer. */

import { de } from "./locale-de.js";
import { en } from "./locale-en.js";

const STORAGE_KEY = "openbyte-language";
const SUPPORTED_LOCALES = new Set(["en", "de"]);
const catalogs = { en, de };
const missingWarnings = new Set();
const formatterCache = new Map();

function normalizeLocale(value) {
  if (typeof value !== "string") return null;
  const base = value.trim().replaceAll("_", "-").toLowerCase().split("-")[0];
  return SUPPORTED_LOCALES.has(base) ? base : null;
}

function queryLocale() {
  try {
    return normalizeLocale(new URL(globalThis.location.href).searchParams.get("lang"));
  } catch {
    return null;
  }
}

function storedLocale() {
  try {
    return normalizeLocale(localStorage.getItem(STORAGE_KEY));
  } catch {
    return null;
  }
}

function browserLocale() {
  const languages = Array.isArray(navigator.languages)
    ? navigator.languages
    : [navigator.language];
  for (const language of languages) {
    const supported = normalizeLocale(language);
    if (supported) return supported;
  }
  return "en";
}

function resolveInitialLocale() {
  const query = queryLocale();
  if (query) return { locale: query, preference: query };
  const stored = storedLocale();
  if (stored) return { locale: stored, preference: stored };
  return { locale: browserLocale(), preference: "auto" };
}

let { locale, preference } = resolveInitialLocale();
document.documentElement.lang = locale;

export function getLocale() {
  return locale;
}

export function getLocalePreference() {
  return preference;
}

export function t(key, variables = {}) {
  const template = catalogs[locale]?.[key] ?? en[key];
  if (typeof template !== "string") {
    if (!missingWarnings.has(key)) {
      missingWarnings.add(key);
      console.warn(`Missing translation key: ${key}`);
    }
    return "";
  }
  return template.replace(/\{([a-zA-Z][a-zA-Z0-9]*)\}/g, (match, name) =>
    Object.hasOwn(variables, name) ? String(variables[name]) : match,
  );
}

function cachedFormatter(type, options) {
  const cacheKey = `${type}:${locale}:${JSON.stringify(options)}`;
  if (!formatterCache.has(cacheKey)) {
    const constructors = {
      number: Intl.NumberFormat,
      date: Intl.DateTimeFormat,
      relative: Intl.RelativeTimeFormat,
    };
    formatterCache.set(cacheKey, new constructors[type](locale, options));
  }
  return formatterCache.get(cacheKey);
}

export function formatNumber(value, options = {}) {
  return cachedFormatter("number", options).format(value);
}

export function formatDateTime(value, options = {}) {
  return cachedFormatter("date", options).format(value);
}

export function formatRelativeTime(value, unit, options = {}) {
  return cachedFormatter("relative", options).format(value, unit);
}

function translateText(root, selector, attribute) {
  for (const element of root.querySelectorAll(selector)) {
    const key = element.dataset[attribute];
    if (key) element.textContent = t(key);
  }
}

function translateAttribute(root, selector, datasetKey, attribute) {
  for (const element of root.querySelectorAll(selector)) {
    const key = element.dataset[datasetKey];
    if (key) element.setAttribute(attribute, t(key));
  }
}

function localizedURL(value) {
  const url = new URL(value, globalThis.location.href);
  if (preference === "auto") url.searchParams.delete("lang");
  else url.searchParams.set("lang", locale);
  return url;
}

export function localizeURL(value) {
  return localizedURL(value).toString();
}

function syncLocalizedLinks(root) {
  for (const link of root.querySelectorAll("a[data-locale-link]")) {
    link.href = localizedURL(link.getAttribute("href")).toString();
  }
}

function syncLanguageControl(root) {
  const control = root.querySelector("#languageSelect");
  if (!control) return;
  const systemOption = control.querySelector('option[value="auto"]');
  if (systemOption) {
    systemOption.textContent = t("language.system", {
      locale: browserLocale().toUpperCase(),
    });
  }
  control.value = preference;
}

export function localizeDocument(root = document) {
  document.documentElement.lang = locale;
  translateText(root, "[data-i18n]", "i18n");
  translateAttribute(
    root,
    "[data-i18n-aria-label]",
    "i18nAriaLabel",
    "aria-label",
  );
  translateAttribute(root, "[data-i18n-title]", "i18nTitle", "title");
  translateAttribute(root, "[data-i18n-content]", "i18nContent", "content");
  syncLocalizedLinks(root);
  syncLanguageControl(root);
}

function persistPreference(nextPreference) {
  try {
    if (nextPreference === "auto") localStorage.removeItem(STORAGE_KEY);
    else localStorage.setItem(STORAGE_KEY, nextPreference);
  } catch {
    // Storage may be blocked; the language still applies to this page.
  }
}

function removeQueryOverride() {
  try {
    const url = new URL(globalThis.location.href);
    if (!url.searchParams.has("lang")) return;
    url.searchParams.delete("lang");
    history.replaceState(history.state, "", url);
  } catch {
    // A malformed/non-browser location should not block language switching.
  }
}

export function setLocalePreference(nextPreference) {
  if (nextPreference !== "auto" && !SUPPORTED_LOCALES.has(nextPreference)) {
    return false;
  }
  preference = nextPreference;
  locale = nextPreference === "auto" ? browserLocale() : nextPreference;
  persistPreference(nextPreference);
  removeQueryOverride();
  localizeDocument();
  document.dispatchEvent(
    new CustomEvent("openbyte:localechange", { detail: { locale } }),
  );
  return true;
}

export function onLocaleChange(listener) {
  const handler = (event) => listener(event.detail.locale);
  document.addEventListener("openbyte:localechange", handler);
  return () => document.removeEventListener("openbyte:localechange", handler);
}

function wireLanguageControl() {
  const control = document.getElementById("languageSelect");
  if (!control || control.dataset.localeWired === "true") return;
  control.dataset.localeWired = "true";
  control.value = preference;
  control.addEventListener("change", () => {
    setLocalePreference(control.value);
  });
}

function init() {
  localizeDocument();
  wireLanguageControl();
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init, { once: true });
} else {
  init();
}
