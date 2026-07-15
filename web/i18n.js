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
  const stored = storedLocale();
  if (stored) return { locale: stored, preference: stored };
  return { locale: browserLocale(), preference: "auto" };
}

const { locale, preference } = resolveInitialLocale();
document.documentElement.lang = locale;

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
  const cacheKey = `${type}:${JSON.stringify(options)}`;
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

function translateAriaLabels(root) {
  for (const element of root.querySelectorAll("[data-i18n-aria-label]")) {
    const key = element.dataset.i18nAriaLabel;
    if (key) element.setAttribute("aria-label", t(key));
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

function localizeDocument(root = document) {
  document.documentElement.lang = locale;
  translateText(root, "[data-i18n]", "i18n");
  translateAriaLabels(root);
  syncLanguageControl(root);
}

function persistPreference(nextPreference) {
  try {
    if (nextPreference === "auto") localStorage.removeItem(STORAGE_KEY);
    else localStorage.setItem(STORAGE_KEY, nextPreference);
    return true;
  } catch {
    return false;
  }
}

function wireLanguageControl() {
  const control = document.getElementById("languageSelect");
  if (!control || control.dataset.localeWired === "true") return;
  control.dataset.localeWired = "true";
  control.value = preference;
  control.addEventListener("change", () => {
    const nextPreference = control.value;
    if (
      (nextPreference === "auto" || SUPPORTED_LOCALES.has(nextPreference)) &&
      persistPreference(nextPreference)
    ) {
      globalThis.location.reload();
      return;
    }
    control.value = preference;
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
