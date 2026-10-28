# Main design: V2

V2 is the default interface at `/`. The original visual styling is retained at
`/?design=v1`; `/?design=v2` also continues to show V2.

The sidebar is the main navigation point. Workspace placeholders, slogan cards,
theme-preview controls, repeated navigation shortcuts, the activity retention
explanation strip, and demo banners/footers have been removed.

Secondary interface text uses a minimum 12px size in V2. The server information
card explicitly applies this to interface/status/version/uptime, public keys,
and copy controls. Its description is 13px and its heading is 16px.

## Runtime

The executable defaults to `--mode live`, which requires Linux. The installer
and systemd service already explicitly select live mode. Real installation
instructions are in `INSTALL.md`.

Demo mode remains available only when explicitly requested with `--mode demo`.
The Windows scripts `start-demo.ps1` and `start-design-preview.ps1`, and the
local `npm run dev` workflow, explicitly use that mode for development. Removing
visual demo labels does not turn a Windows demo into a real WireGuard server.
The preview script uses its own `.data/design-preview-v2` directory.

## Implementation

- `ui/index.html` loads the shared styles followed by the main V2 stylesheet.
- `ui/design-v2.css` contains the main visual styling.
- `ui/design.ts` removes V2 styles only for an explicit `?design=v1` request.
- Both designs share application markup, API calls, validation, permissions,
  and event handlers in `ui/app.ts`.
- Client management, QR codes, downloads, imports, monitoring, metrics,
  panel-access policy, and authentication retain their existing behavior.
