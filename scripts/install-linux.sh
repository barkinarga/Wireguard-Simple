#!/usr/bin/env bash
# Compatibility entry point; the maintained installer is install.sh.
set -euo pipefail
exec bash "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)/install.sh" "$@"
