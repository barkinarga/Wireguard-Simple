# Interface languages

The application defaults to English regardless of the browser or operating system language. A user can explicitly choose one of 10 languages in the top bar or before administrator setup/sign-in. Only that language code is saved as `wgs.language` in browser local storage. A new browser starts in English; inaccessible or unsupported saved preferences fall back to English.

| Code | Interface language |
| --- | --- |
| `en` | English |
| `tr` | Turkish |
| `de` | German |
| `fr` | French |
| `es` | Spanish |
| `it` | Italian |
| `pt` | Portuguese (Brazilian formatting) |
| `ru` | Russian |
| `ar` | Arabic, right-to-left layout |
| `zh` | Simplified Chinese |

All dictionaries are bundled into the compiled interface, embedded in the Go binary. No translation network requests, services, accounts, or extra server processes are needed. Dates, numeric values, relative times, and uptime use browser `Intl` formatting. Client names, public keys, addresses, and WireGuard configuration content are treated as user data and are never translated.

`ui/i18n.ts` selects the dictionary and controls the document's language/direction. `ui/locales/en.json` supplies typed English message identifiers; each other dictionary contains the same identifiers and interpolation fields. A missing translation falls back to English. `server-messages.json` maps legacy Turkish server messages and parameterized events to their canonical English identifiers, so old administrative history remains readable in the selected language. Interpolated user data is escaped at the HTML rendering boundary.

Common validation errors and administrative events are localized. Low-level operating system, configuration-parser and WireGuard diagnostics retain English technical detail. Installer terminal output is English. Repository-facing prose remains English and Turkish.

When adding a language, add its JSON dictionary, register its native name, formatting locale and direction in `languages`, then run:

```bash
npm run check
npm run build
npm run test:i18n
```

The tests check message completeness, interpolation fields, English defaults, storage failures, preference persistence, RTL reset, literal user data, legacy/current events, and the compressed interface size budget. Also check long labels and right-to-left layout in desktop/mobile browsers. Browser-native form validation messages follow the browser's own language.
