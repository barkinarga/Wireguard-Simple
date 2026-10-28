import en from './locales/en.json';
import tr from './locales/tr.json';
import de from './locales/de.json';
import fr from './locales/fr.json';
import es from './locales/es.json';
import it from './locales/it.json';
import pt from './locales/pt.json';
import ru from './locales/ru.json';
import ar from './locales/ar.json';
import zh from './locales/zh.json';
import legacy from './locales/server-messages.json';

export const languages = [
  { code: 'en', name: 'English', locale: 'en-US', dir: 'ltr' },
  { code: 'tr', name: 'Türkçe', locale: 'tr-TR', dir: 'ltr' },
  { code: 'de', name: 'Deutsch', locale: 'de-DE', dir: 'ltr' },
  { code: 'fr', name: 'Français', locale: 'fr-FR', dir: 'ltr' },
  { code: 'es', name: 'Español', locale: 'es-ES', dir: 'ltr' },
  { code: 'it', name: 'Italiano', locale: 'it-IT', dir: 'ltr' },
  { code: 'pt', name: 'Português', locale: 'pt-BR', dir: 'ltr' },
  { code: 'ru', name: 'Русский', locale: 'ru-RU', dir: 'ltr' },
  { code: 'ar', name: 'العربية', locale: 'ar', dir: 'rtl' },
  { code: 'zh', name: '简体中文', locale: 'zh-CN', dir: 'ltr' },
] as const;

export type Language = typeof languages[number]['code'];
export type MessageKey = keyof typeof en;
type Values = Record<string, string | number>;
type PreferenceStore = Pick<Storage, 'getItem' | 'setItem'>;
const catalogs: Record<Language, Partial<Record<MessageKey, string>>> = { en, tr, de, fr, es, it, pt, ru, ar, zh };
export const preferenceKey = 'wgs.language';
let current: Language = 'en';

export function resolveLanguage(value: unknown): Language {
  return languages.find(item => item.code === value)?.code ?? 'en';
}
export function language(): Language { return current; }
export function locale(): string { return languages.find(item => item.code === current)!.locale; }

export function initLanguage(storage?: PreferenceStore): void {
  let saved: string | null = null;
  try { saved = (storage ?? window.localStorage).getItem(preferenceKey); } catch { /* Storage can be unavailable in private browsers. */ }
  setLanguage(saved, false, storage);
}

export function setLanguage(value: unknown, persist = true, storage?: PreferenceStore): void {
  current = resolveLanguage(value);
  if (typeof document !== 'undefined') {
    document.documentElement.lang = current;
    document.documentElement.dir = languages.find(item => item.code === current)!.dir;
  }
  if (persist) {
    try { (storage ?? window.localStorage).setItem(preferenceKey, current); } catch { /* The current page still works without persistence. */ }
  }
}

function interpolate(message: string, values: Values): string {
  return message.replace(/\{([a-zA-Z]+)\}/g, (placeholder, name: string) => {
    const value = values[name];
    return value === undefined ? placeholder : typeof value === 'number' ? new Intl.NumberFormat(locale()).format(value) : value;
  });
}
export function t(key: MessageKey, values: Values = {}): string {
  return interpolate(catalogs[current][key] || en[key], values);
}

// Only server-authored messages pass through this compatibility layer. User
// names, addresses, keys and configuration contents are never translated.
const aliases = legacy as Record<string, string>;
const patterns = Object.entries(aliases).filter(([source]) => source.includes('{value}')).map(([source, target]) => ({
  pattern: new RegExp('^' + source.split('{value}').map(part => part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('(.*)') + '$'), target,
}));
export function serverMessage(value: unknown): string {
  const message = typeof value === 'string' ? value : en['Request failed'];
  const canonical = Object.hasOwn(aliases, message) ? aliases[message] : message;
  if (Object.hasOwn(en, canonical)) return t(canonical as MessageKey);
  for (const { pattern, target } of patterns) {
    const match = pattern.exec(message);
    if (match) return Object.hasOwn(en, target) ? t(target as MessageKey, { value: match[1] }) : interpolate(target, { value: match[1] });
  }
  // Low-level OS/WireGuard diagnostics retain their original technical detail.
  return canonical;
}

export function duration(startedAt: string): string {
  const seconds = Math.max(0, (Date.now() - new Date(startedAt).getTime()) / 1000);
  const [value, unit] = seconds < 60 ? [seconds, 'second'] : seconds < 3600 ? [seconds / 60, 'minute'] : seconds < 86400 ? [seconds / 3600, 'hour'] : [seconds / 86400, 'day'];
  return new Intl.NumberFormat(locale(), { style: 'unit', unit: String(unit), unitDisplay: 'short', maximumFractionDigits: 0 }).format(Math.floor(Number(value)));
}
