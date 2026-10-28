# WireGuard Simple v0.4.1

Use the same one-line command to install or update. From this version, an already downloaded installer also checks GitHub for the latest stable release before installing.

- A newer release is downloaded with its checksum from one pinned release tag and verified before execution.
- An already-current or newer installed version is left running on a default invocation; no service restart or downgrade. Explicit configuration options still run.
- Failed release checks, downloads and checksum validation stop before installation. The running application remains unchanged.
- Existing accounts, client keys, expiry dates, settings and service configuration retain the established upgrade behavior.
- `--check` makes no release request. `--no-update-check` explicitly uses the embedded package for offline or pinned installs. Delegated updates cannot recurse.
- V2 remains the default interface from v0.4.0, with the existing ten languages and original design available at `/?design=v1`.

Installer regression tests cover numeric version ordering, corrupt/duplicate checksums, failed downloads, unavailable or invalid releases, argument preservation, recursion prevention, no-op updates and install-error propagation. No backup feature or background update scheduler is added.
