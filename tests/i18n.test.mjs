import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { gzipSync } from 'node:zlib';
import { build } from 'esbuild';

const bundled = await build({ entryPoints: ['ui/i18n.ts'], bundle: true, write: false, platform: 'node', format: 'esm' });
const i18n = await import('data:text/javascript;base64,' + Buffer.from(bundled.outputFiles[0].contents).toString('base64'));
const en = JSON.parse(fs.readFileSync('ui/locales/en.json', 'utf8'));
const placeholders = value => [...value.matchAll(/\{([a-zA-Z]+)\}/g)].map(m => m[1]).sort();

test('all ten languages have every message and preserve interpolation fields', () => {
  assert.equal(i18n.languages.length, 10);
  for (const { code } of i18n.languages) {
    const catalog = JSON.parse(fs.readFileSync(`ui/locales/${code}.json`, 'utf8'));
    assert.deepEqual(Object.keys(catalog).sort(), Object.keys(en).sort(), code);
    for (const [key, value] of Object.entries(catalog)) {
      assert.equal(typeof value, 'string', `${code}: ${key}`);
      assert.ok(value.trim(), `${code}: ${key}`);
      assert.deepEqual(placeholders(value), placeholders(en[key]), `${code}: ${key}`);
      assert.doesNotMatch(value, /[<>]/, `${code}: translations are text, not markup`);
    }
  }
});

test('fresh, unsupported and unavailable preferences all default to English', () => {
  for (const saved of [null, '', 'xx', '__proto__', '{"locale":"tr"}']) {
    i18n.initLanguage({ getItem: () => saved, setItem: () => assert.fail('initialization must not write') });
    assert.equal(i18n.language(), 'en');
    assert.equal(i18n.t('Overview'), 'Overview');
  }
  i18n.initLanguage({ getItem: () => { throw Error('storage disabled'); }, setItem() {} });
  assert.equal(i18n.language(), 'en');
});

test('only the chosen language code is persisted and restored', () => {
  const values = new Map();
  const storage = { getItem: key => values.get(key) ?? null, setItem: (key, value) => values.set(key, value) };
  i18n.setLanguage('tr', true, storage);
  assert.deepEqual([...values], [['wgs.language', 'tr']]);
  i18n.setLanguage('en', false);
  i18n.initLanguage(storage);
  assert.equal(i18n.t('Overview'), 'Genel bakış');
  i18n.setLanguage('fr', true, { getItem() {}, setItem() { throw Error('storage full'); } });
  assert.equal(i18n.t('Overview'), 'Vue d’ensemble');
});

test('language and direction switch back from Arabic to English', () => {
  globalThis.document = { documentElement: {} };
  try {
    i18n.setLanguage('ar', false);
    assert.deepEqual(document.documentElement, { lang: 'ar', dir: 'rtl' });
    assert.equal(i18n.locale(), 'ar');
    i18n.setLanguage('en', false);
    assert.deepEqual(document.documentElement, { lang: 'en', dir: 'ltr' });
  } finally { delete globalThis.document; }
});

test('interpolation preserves names literally and formats numeric values per locale', () => {
  const name = '$& <img src=x> {count}';
  i18n.setLanguage('de', false);
  assert.equal(i18n.t('Details for {name}', { name }), `Details zu ${name}`);
  assert.equal(i18n.t('{count} samples · RAM buffer', { count: 1234 }), '1.234 Messungen · RAM-Puffer');
  i18n.setLanguage('en', false);
  assert.equal(i18n.t('{count} samples · RAM buffer', { count: 1234 }), '1,234 samples · RAM buffer');
});

test('new and legacy server events localize while preserving client names', () => {
  i18n.setLanguage('es', false);
  assert.equal(i18n.serverMessage('Invalid setup token'), 'Token de configuración no válido');
  assert.equal(i18n.serverMessage('kurulum anahtarı geçersiz'), 'Token de configuración no válido');
  assert.equal(i18n.serverMessage('İstemci eklendi: Test <device>'), 'Cliente añadido: Test <device>');
  assert.equal(i18n.serverMessage('Client added: Test <device>'), 'Cliente añadido: Test <device>');
  i18n.setLanguage('en', false);
  assert.equal(i18n.serverMessage('WireGuard yanıtı boyut sınırını aşıyor'), 'WireGuard response exceeds the size limit');
  assert.equal(i18n.serverMessage('wg: operating system diagnostic'), 'wg: operating system diagnostic');
  assert.equal(i18n.serverMessage('__proto__'), '__proto__');
  assert.equal(i18n.serverMessage('constructor'), 'constructor');
  assert.equal(i18n.serverMessage(null), 'Request failed');
});

test('all embedded UI languages fit a 100 KiB compressed script budget', () => {
  const script = fs.readFileSync('internal/web/dist/app.js');
  assert.ok(script.length < 512 * 1024, `uncompressed UI: ${script.length}`);
  assert.ok(gzipSync(script).length < 100 * 1024, `compressed UI: ${gzipSync(script).length}`);
});
