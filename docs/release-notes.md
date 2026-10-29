# WireGuard Simple v0.4.12

Improve readability throughout the existing panel without changing its visual identity or VPN behavior.

- Centralize active text sizes as shared relative tokens: 12px helper/status text, 14px body/actions and 16/18px supporting headings at the default browser font size. Keep mobile inputs at 16px and icon targets at 44px.
- Correct faint storage totals/capacity, client details/public keys, summary counters and passive badges. Apply consistent text, status, disabled, placeholder, focus and destructive-action colors across pages, dialogs and authentication.
- Keep numeric counters tabular, allow long names/keys and translated controls to wrap, and keep narrow layouts within the viewport. Demo free disk space is unavailable rather than a false zero; existing off/paused/empty/loading states remain distinct.
- Preserve table horizontal scroll and page scroll across live refreshes. Defer full content redraw while scrolling, selecting text or focusing a main-content control. Avoid rebuilding unchanged connection-status markup and remove remaining active-theme dialog backdrop blur.
- Show persistent inline form errors, retain inputs after failed submissions, prevent duplicate submissions and expose pending state. Failed sign-in/setup attempts no longer rebuild an already unauthenticated form; authenticated-session expiration still returns to sign-in.

Validation includes TypeScript/build, translation and script-size checks, client-address regressions, real-browser computed text-size/contrast checks across all five pages and principal dialogs/authentication, 320px mobile and 640px reflow, Turkish labels, duplicate-client error handling and saved-input checks. Full build/platform/firewall release gates remain required.
