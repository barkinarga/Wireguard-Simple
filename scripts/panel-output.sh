#!/usr/bin/env bash
# Shared read-only terminal output for installation and already-current checks.
wgs_heading() {
  local color=$1 title=$2
  printf '\n'
  wgs_color "$color" "$title"
}
wgs_color() {
  local color=$1 title=$2
  if [[ -t 1 && ${TERM:-dumb} != dumb && ! -v NO_COLOR ]]; then
    printf '\033[1;%sm%s\033[0m\n' "$color" "$title"
  else
    printf '%s\n' "$title"
  fi
}
wgs_panel_links() {
  local command_line listen=127.0.0.1:8787 host port scheme=https panel_ip unit_dir=${WGS_UNIT_DIR:-/etc/systemd/system}
  # Inspect effective service arguments; never execute or source their contents.
  command_line=$(systemctl show --property=ExecStart --value wgsimple.service 2>/dev/null) || command_line=
  if [[ -z $command_line && -r $unit_dir/wgsimple.service ]]; then
    command_line=$(< "$unit_dir/wgsimple.service")
  fi
  if [[ $command_line =~ --listen[=[:space:]]+([^[:space:]\;]+) ]]; then
    listen=${BASH_REMATCH[1]}
  fi
  if [[ $listen =~ ^(\[[0-9a-fA-F:]+\]|[a-zA-Z0-9._-]*):([0-9]+)$ ]]; then
    host=${BASH_REMATCH[1]}
    port=${BASH_REMATCH[2]}
  else
    printf '%s\n' 'Panel:        See the service listen address.'
    return
  fi
  case $host in
    localhost|127.*|'[::1]')
      if [[ $command_line != *--tls-cert* ]]; then scheme=http; fi
      wgs_color 36 "Local:        $scheme://$host:$port"
      ;;
    ''|0.0.0.0|'[::]')
      panel_ip=$(ip -o -4 addr show scope global 2>/dev/null | awk '{split($4,a,"/"); print a[1]; exit}') || panel_ip=
      if [[ ! $panel_ip =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then panel_ip=SERVER_IP; fi
      wgs_color 36 "Panel:        https://$panel_ip:$port"
      wgs_color 36 "Local:        https://127.0.0.1:$port"
      ;;
    *) wgs_color 36 "Panel:        https://$host:$port" ;;
  esac
}
