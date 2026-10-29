# WireGuard Simple v0.4.11

Make client creation clearer without changing VPN access or routing defaults.

- Prefill the first available client IPv4 /32 from the current server subnet and show the subnet next to it. The address remains editable; clearing it retains automatic allocation. Normalize plain IPv4 input to /32.
- Show inline warnings for occupied, server-owned, reserved and out-of-subnet addresses. Refresh occupied addresses before submission and preserve the form on concurrent conflicts. The existing server-side change lock remains the final allocation authority; a preview does not reserve an address.
- Keep the current palette and typography while increasing field/helper readability and touch targets. Keep the dialog header and actions visible, with one scrollable form body and mobile layout.
- Remove the client dialog backdrop blur and contain form scrolling; no measured refresh-rate claim is made.
- Explain Linux FwMark and automatic per-client preshared keys. Localize new helper/error text in all ten supported languages.

Validation: TypeScript/build and translation/size checks; address preview regressions covering subnet changes, gaps, exhaustion, routed prefixes and stale proposals; existing backend advanced-client validation tests. Browser demo checks include custom-IP creation, two-session address conflicts, desktop and 390px mobile layout/scroll access. No production VPN state was changed for UI testing.
