#!/usr/bin/env bash
# Compatibility entry point; the maintained installer is install.sh.
set -euo pipefail
script_dir=${BASH_SOURCE[0]%/*}
if [[ $script_dir == "${BASH_SOURCE[0]}" ]]; then script_dir=.; fi
exec bash "$script_dir/install.sh" "$@"
