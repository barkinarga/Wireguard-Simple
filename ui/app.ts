import { t, language, locale, languages, setLanguage, initLanguage, serverMessage, duration } from './i18n';
import './design';
import { clientAddressPreview, clientAddressIssue, normalizeClientAddress } from './client-address';
export {};
type Settings = { historyEnabled: boolean; auditEnabled: boolean; metricsEnabled: boolean; retentionDays: number; budgetMB: number; sampleSeconds: number };
type Server = { name: string; interface: string; address: string; endpoint: string; port: number; dns: string; allowedIPs: string; mtu: number; publicKey: string };
type Peer = { id: string; name: string; address: string; publicKey: string; enabled: boolean; createdAt: string; expiresAt: string | null; hasConfig: boolean; status: string; endpoint: string; lastHandshake: string | null; rxBytes: number; txBytes: number; rxRate: number; txRate: number };
type Point = { time: string; rx: number; tx: number; active: number };
type Event = { id: string; time: string; level: string; action: string; detail: string };
type Resources = { appMemory: number; goroutines: number; hostMemoryTotal: number; hostMemoryUsed: number; diskFree: number; diskTotal: number; hostCPU: number; controlBytes: number; telemetryBytes: number };
type Snapshot = { mode: string; version: string; startedAt: string; updatedAt: string; server: Server; settings: Settings; peers: Peer[]; points: Point[]; events: Event[]; resources: Resources; warnings: string[]; interfaceUp: boolean; historyPaused: boolean };
type Session = { authenticated: boolean; setupRequired: boolean; mode: string; user: string; csrf: string };
type View = 'overview' | 'peers' | 'activity' | 'storage' | 'settings';

const app = document.querySelector<HTMLDivElement>('#app')!;
const modal = document.querySelector<HTMLDialogElement>('#modal')!;
const paths: Record<string, string> = {
  grid: '<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>',
  devices: '<rect x="3" y="4" width="14" height="11" rx="2"/><path d="M7 20h6m-3-5v5"/><rect x="17" y="10" width="5" height="11" rx="1"/>',
  pulse: '<path d="M2 12h5l3-8 4 16 3-8h5"/>',
  storage: '<ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v7c0 1.7 3.6 3 8 3s8-1.3 8-3V5M4 12v7c0 1.7 3.6 3 8 3s8-1.3 8-3v-7"/>',
  settings: '<path d="M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8Z"/><path d="m9 3-1 3-3 1-2 3 2 2-1 4 3 2 3-1 2 4 3-1 1-3 4-1 1-3-3-2 1-3-3-2-3 1Z"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  arrow: '<path d="M5 12h14m-5-5 5 5-5 5"/>',
  down: '<path d="M12 4v12m-5-5 5 5 5-5M4 17v4h16v-4"/>',
  up: '<path d="M12 20V4m-6 6 6-6 6 6"/>',
  chevron: '<path d="m9 5 7 7-7 7"/>',
  search: '<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/>',
  shield: '<path d="m12 3 8 3v6c0 5-8 9-8 9S4 17 4 12V6l8-3Z"/><path d="m8 12 3 3 5-6"/>',
  server: '<rect x="3" y="3" width="18" height="7" rx="2"/><rect x="3" y="14" width="18" height="7" rx="2"/><path d="M7 6.5h.1M7 17.5h.1M16 6.5h2M16 17.5h2"/>',
  leaf: '<path d="M20 3c-9-1-17 4-16 11 1 8 15 9 16-11ZM5 19 16 8"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  close: '<path d="m6 6 12 12M6 18 18 6"/>',
  check: '<path d="m5 12 4 4L19 6"/>',
  dots: '<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
  qr: '<rect x="3" y="3" width="6" height="6"/><rect x="15" y="3" width="6" height="6"/><rect x="3" y="15" width="6" height="6"/><path d="M15 15h3v3h3v3h-6v-3m6-3v-3M12 3v6m-9 3h6m3 3v6"/>',
  pause: '<path d="M8 5v14M16 5v14"/>',
  play: '<path d="m8 4 12 8-12 8V4Z"/>',
  trash: '<path d="M3 6h18M9 6V3h6v3M6 6l1 15h10l1-15M10 10v7m4-7v7"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v6m0-10v.1"/>',
  copy: '<rect x="8" y="8" width="12" height="13" rx="2"/><path d="M15 8V3H3v12h5"/>',
  bell: '<path d="M5 10a7 7 0 0 1 14 0v5l2 3H3l2-3v-5Zm5 11h4"/>',
  globe: '<circle cx="12" cy="12" r="9"/><ellipse cx="12" cy="12" rx="4" ry="9"/><path d="M3 12h18"/>',
  logout: '<path d="M9 4H4v16h5m5-12 4 4-4 4M8 12h12"/>',
  menu: '<path d="M4 6h16M4 12h16M4 18h16"/>',
  lock: '<rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3m-4 5v2"/>',
  upload: '<path d="M12 16V3m-5 5 5-5 5 5M4 16v5h16v-5"/>',
};
function icon(name: string, cls = ''): string { return `<svg class="icon ${cls}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths[name] ?? paths.info}</svg>`; }
function esc(v: unknown): string { return String(v ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]!); }
function bytes(v: number, decimals = 1): string { if (!Number.isFinite(v) || v <= 0) return '0 B'; const i = Math.min(4, Math.floor(Math.log(v) / Math.log(1024))); return `${(v / 1024 ** i).toLocaleString(locale(), { maximumFractionDigits: decimals })} ${['B', 'KB', 'MB', 'GB', 'TB'][i]}`; }
function rate(v: number): string { if (v < 125000) return `${(v * 8 / 1000).toLocaleString(locale(), { maximumFractionDigits: 1 })} Kbps`; return `${(v * 8 / 1e6).toLocaleString(locale(), { maximumFractionDigits: 2 })} Mbps`; }
function ago(v: string | null): string { if (!v) return t('Not yet'); const seconds = Math.max(0, (Date.now() - new Date(v).getTime()) / 1000); const [value, unit] = seconds < 60 ? [seconds, 'second'] : seconds < 3600 ? [seconds / 60, 'minute'] : seconds < 86400 ? [seconds / 3600, 'hour'] : [seconds / 86400, 'day']; return new Intl.RelativeTimeFormat(locale(), { numeric: 'always', style: 'short' }).format(-Math.floor(Number(value)), unit as Intl.RelativeTimeFormatUnit); }
function time(v: string): string { return new Date(v).toLocaleTimeString(locale(), { hour: '2-digit', minute: '2-digit' }); }
function date(v: string): string { return new Date(v).toLocaleDateString(locale(), { day: 'numeric', month: 'short' }); }
function status(p: Peer): string { return p.status === 'active' ? `<span class="badge green"><i></i>${t("Active")}</span>` : p.status === 'disabled' ? `<span class="badge muted"><i></i>${t("Disabled")}</span>` : `<span class="badge amber"><i></i>${t("Idle")}</span>`; }

let session: Session;
let state: Snapshot;
let view: View = 'overview';
let query = '';
let peerFilter = 'all';
let chartMode = 'live';
let history: Point[] = [];
let poll: ReturnType<typeof setInterval> | undefined;
let refreshBusy = false;
let scrollIdleAt = 0;
document.addEventListener('scroll', () => { scrollIdleAt = performance.now() + 250; }, { passive: true, capture: true });
let failures = 0;
let toastTimer: ReturnType<typeof setTimeout>;
let qrBlob = '';

async function api<T = unknown>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const response = await fetch(path, { method, credentials: 'same-origin', headers: { ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}), ...(session?.csrf ? { 'X-CSRF-Token': session.csrf } : {}) }, body: body === undefined ? undefined : JSON.stringify(body) });
  if (!response.ok) { const error = await response.json().catch(() => ({ error: `${t("Request failed")} (${response.status})` })); if (response.status === 401 && session?.authenticated) { clearInterval(poll); session.authenticated = false; renderAuth(); } throw new Error(serverMessage(error.error ?? t("Request failed"))); }
  return await response.json() as T;
}
function toast(message: string, error = false): void { const el = document.querySelector<HTMLDivElement>('#toast')!; el.textContent = message; el.className = `visible ${error ? 'error' : ''}`; clearTimeout(toastTimer); toastTimer = setTimeout(() => { el.className = ''; }, 4500); }
async function action(fn: () => Promise<void>): Promise<void> { try { await fn(); } catch (err) { toast(err instanceof Error ? err.message : t("Action failed"), true); } }
async function refresh(force = false): Promise<void> {
  if (refreshBusy || (document.hidden && !force)) return; refreshBusy = true;
  try {
    state = await api<Snapshot>('/api/snapshot'); failures = 0;
    if (!document.querySelector('.shell')) shell();
    const active = document.activeElement;
    const editing = active instanceof Element && Boolean(active.closest('input,textarea,select,button,a,[tabindex]')?.closest('#main'));
    const selecting = Boolean(window.getSelection()?.toString());
    if (force || !modal.open && !editing && !selecting && performance.now() >= scrollIdleAt && ['overview', 'peers', 'activity'].includes(view)) {
      const left = Array.from(document.querySelectorAll('.table-scroll')).map(el => el.scrollLeft);
      const top = window.scrollY;
      renderMain();
      document.querySelectorAll('.table-scroll').forEach((el, i) => { el.scrollLeft = left[i] ?? 0; });
      window.scrollTo({ top, behavior: 'instant' });
    }
    updateChrome();
  } catch (error) { failures++; updateChrome(); if (force) throw error; } finally { refreshBusy = false; }
}
function navigate(next: View): void { view = next; query = ''; chartMode = 'live'; location.hash = next; renderMain(); updateChrome(); document.querySelector('.sidebar')?.classList.remove('mobile-open'); document.querySelector('#mobile-menu')?.setAttribute('aria-expanded','false'); window.scrollTo(0, 0); }

function languageControl(): string {
  return `<label class="language-control">${icon('globe')}<span class="sr-only">${t('Language')}</span><select id="language-picker" aria-label="${t('Language')}">${languages.map(item => `<option value="${item.code}" lang="${item.code}" dir="auto" ${item.code === language() ? 'selected' : ''}>${item.name}</option>`).join('')}</select></label>`;
}
function bindLanguageControl(): void {
  document.querySelector<HTMLSelectElement>('#language-picker')?.addEventListener('change', event => {
    const selection = (event.currentTarget as HTMLSelectElement).value;
    // Preserve unsaved input during the local re-render; never save form data
    // or passwords to browser storage. Only the language code is persisted.
    const drafts = Array.from(app.querySelectorAll<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>('form[id] [name]')).map(field => ({ form: field.form?.id, name: field.name, value: field.value, checked: field instanceof HTMLInputElement && field.checked }));
    const scroll = window.scrollY;
    setLanguage(selection);
    if (session?.authenticated && state) { shell(); renderMain(); updateChrome(); } else { renderAuth(); }
    for (const draft of drafts) {
      const field = document.getElementById(draft.form ?? '') instanceof HTMLFormElement ? (document.getElementById(draft.form!) as HTMLFormElement).elements.namedItem(draft.name) : null;
      if (field instanceof HTMLInputElement || field instanceof HTMLSelectElement || field instanceof HTMLTextAreaElement) {
        field.value = draft.value;
        if (draft.form === "panel-access-form") field.form!.dataset.restored = "true";
        if (field instanceof HTMLInputElement && ['checkbox', 'radio'].includes(field.type)) field.checked = draft.checked;
      }
    }
    window.scrollTo(0, scroll);
    document.querySelector<HTMLSelectElement>('#language-picker')?.focus({ preventScroll: true });
    toast(t('Language changed'));
  });
}

function shell(): void {
  app.innerHTML = `<div class="shell"><aside class="sidebar"><a href="#overview" class="brand" aria-label="${t("Home")}"><span class="brand-icon"><svg viewBox="0 0 40 40" fill="none"><path d="m5 12 7 17 8-13 8 13 7-17" stroke="currentColor" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round"/></svg></span><span>wireguard<span class="brand-sub">simple</span></span><span class="version-tag">α</span></a><div class="nav-label">${t("MANAGEMENT")}</div><nav aria-label="${t("Main navigation")}">${(['overview', 'peers', 'activity', 'storage', 'settings'] as View[]).map((v, i) => `<button class="nav-item" data-view="${v}">${icon(['grid', 'devices', 'pulse', 'storage', 'settings'][i])}<span>${[t("Overview"), t("Clients"), t("Activity"), t("Storage & monitoring"), t("Settings")][i]}</span>${v === 'peers' ? '<span id="peer-nav-count" class="nav-count">0</span>' : ''}</button>`).join('')}</nav><div class="sidebar-bottom"><div class="sidebar-status"><i class="live-dot"></i><span>${t("Local console")}</span><span id="version"></span></div><div class="profile"><span class="avatar">A</span><span><strong>${t("Administrator")}</strong><small id="profile-user">${t("Administrator account")}</small></span></div></div></aside><div class="main-wrap"><header class="topbar"><button class="icon-btn mobile-menu" id="mobile-menu" aria-label="${t("Open menu")}">${icon('menu')}</button><div class="breadcrumb"><strong id="breadcrumb-current">${t("Overview")}</strong></div><div class="topbar-right">${languageControl()}<span id="connection" class="connection"><i class="live-dot"></i>${t("Live connection")}</span></div></header><main id="main"></main><footer><span>WireGuard Simple</span></footer></div></div>`;
  bindLanguageControl();
  document.querySelectorAll<HTMLButtonElement>('[data-view]').forEach(button => button.addEventListener('click', () => navigate(button.dataset.view as View)));
  document.querySelector('.brand')?.addEventListener('click', event => { event.preventDefault(); navigate('overview'); });
  document.querySelector('#mobile-menu')?.addEventListener('click', () => { const opened = document.querySelector('.sidebar')?.classList.toggle('mobile-open'); document.querySelector('#mobile-menu')?.setAttribute('aria-expanded', String(opened)); });
}
function updateChrome(): void {
  if (!state || !document.querySelector('.shell')) return;
  const names: Record<View, string> = { overview: t("Overview"), peers: t("Clients"), activity: t("Activity"), storage: t("Storage & monitoring"), settings: t("Settings") };
  document.querySelectorAll<HTMLButtonElement>('.nav-item').forEach(button => { button.classList.toggle('active', button.dataset.view === view); if (button.dataset.view === view) button.setAttribute('aria-current', 'page'); else button.removeAttribute('aria-current'); });
  document.querySelector('#breadcrumb-current')!.textContent = names[view]; document.title = `${names[view]} · WireGuard Simple`;
  document.querySelector('#peer-nav-count')!.textContent = String(state.peers.length);
  document.querySelector('#version')!.textContent = `v${state.version.replace('-dev', '')}`;
  document.querySelector('#profile-user')!.textContent = state.mode === 'demo' ? t('Administrator') : session.user;
  const connection = document.querySelector<HTMLElement>('#connection')!;
  const connectionState = `${language()}:${failures > 0}`;
  if (connection.dataset.state !== connectionState) {
    connection.innerHTML = failures ? `${icon('info')}${t("Reconnecting")}` : `<i class="live-dot"></i>${t("Live connection")}`;
    connection.dataset.state = connectionState;
  }
  document.querySelector('#connection')!.classList.toggle('offline', failures > 0);
}
function pageHead(eyebrow: string, title: string, subtitle: string, buttons = ''): string { return `<div class="page-head"><div><div class="eyebrow">${eyebrow}</div><h1>${title}</h1><p>${subtitle}</p></div><div class="page-actions">${buttons}</div></div>`; }
function addButton(): string { return `<button class="btn primary" data-action="add">${icon('plus')}${t("Add client")}</button>`; }
function empty(iconName: string, title: string, text: string, button = ''): string { return `<div class="empty">${icon(iconName)}<h3>${title}</h3><p>${text}</p>${button}</div>`; }

function renderMain(): void {
  if (!state) return;
  const main = document.querySelector<HTMLElement>('#main')!;
  if (view === 'overview') main.innerHTML = overview();
  else if (view === 'peers') main.innerHTML = peersPage();
  else if (view === 'activity') main.innerHTML = activityPage();
  else if (view === 'storage') main.innerHTML = storagePage();
  else main.innerHTML = settingsPage();
  bindMain();
}
function overview(): string {
  const active = state.peers.filter(p => p.status === 'active').length;
  const rx = state.peers.reduce((n, p) => n + p.rxRate, 0), tx = state.peers.reduce((n, p) => n + p.txRate, 0);
  const total = state.peers.reduce((n, p) => n + p.rxBytes + p.txBytes, 0);
  return `${pageHead(t("NETWORK CONSOLE"), t("Your network, at a glance."), t("Connections, traffic, and server health. All in one place."), `<span class="date-chip">${icon('clock')}${new Date().toLocaleDateString(locale(), { day: 'numeric', month: 'long', year: 'numeric' })}</span>${addButton()}`)}
  ${state.warnings.length ? `<div class="warning-bar">${icon('info')}<div>${state.warnings.map(message => esc(serverMessage(message))).join('<br>')}</div></div>` : ''}
  <div class="stat-grid"><article class="stat-card"><div class="stat-top"><span>${t("Active clients")}</span><span class="stat-icon">${icon('devices')}</span></div><div class="stat-value">${active}<span>/ ${state.peers.length}</span></div><div class="stat-foot"><span class="tiny-dot green-dot"></span>${t("Handshake within 3 minutes")}</div><svg class="stat-decoration" viewBox="0 0 100 30"><path d="m0 26 12-5 13 3 10-10 11 3 10-7 13 2L83 4l17 4"/></svg></article>
  <article class="stat-card"><div class="stat-top"><span>${t("Current traffic")}</span><span class="stat-icon">${icon('pulse')}</span></div><div class="stat-value compact">${rate(rx + tx).split(' ')[0]}<span>${rate(rx + tx).split(' ')[1]}</span></div><div class="stat-foot"><span class="rx-text">↓ ${rate(rx)}</span><span class="tx-text">↑ ${rate(tx)}</span></div><div class="mini-bars" aria-hidden="true">${[4, 2, 5, 3, 6, 4, 7, 5, 8, 6, 9, 7].map(n => `<i class="bar-${n}"></i>`).join('')}</div></article>
  <article class="stat-card"><div class="stat-top"><span>${t("Transferred data")}</span><span class="stat-icon">${icon('down')}</span></div><div class="stat-value compact">${bytes(total).split(' ')[0]}<span>${bytes(total).split(' ')[1]}</span></div><div class="stat-foot">${t("Current WireGuard counters")}</div><span class="stat-note">RX + TX</span></article>
  <article class="stat-card"><div class="stat-top"><span>${t("Application memory")}</span><span class="stat-icon">${icon('leaf')}</span></div><div class="stat-value compact">${bytes(state.resources.appMemory).split(' ')[0]}<span>${bytes(state.resources.appMemory).split(' ')[1]}</span></div><div class="stat-foot"><span class="tiny-dot green-dot"></span>${state.settings.historyEnabled ? t("History recording on") : t("Persistent traffic recording off")}</div><span class="soft-tag">${t("LIGHTWEIGHT CORE")}</span></article></div>
  <div class="dashboard-columns"><div class="dashboard-primary"><section class="panel traffic-panel"><div class="panel-head"><div><h2>${t("Live traffic")} <span class="badge subtle">${icon('pulse')}${t("LIVE")}</span></h2><p>${t("Combined traffic from all clients")}</p></div><div class="segmented" aria-label="${t("Chart source")}"><button class="${chartMode === 'live' ? 'selected' : ''}" data-chart="live">${t("Live")}</button><button class="${chartMode === 'history' ? 'selected' : ''}" data-chart="history">${t("History")} ${!state.settings.historyEnabled ? icon('lock') : ''}</button></div></div>${chartMode === 'history' && !state.settings.historyEnabled ? `<div class="chart-locked">${icon('storage')}<h3>${t("History is your choice.")}</h3><p>${t("Live monitoring does not write to disk. Enable history with your own retention and storage limits.")}</p></div>` : graph(chartMode === 'history' ? history : state.points)}<div class="chart-footer"><span><i class="legend rx"></i>${t("Received (RX)")}</span><span><i class="legend tx"></i>${t("Sent (TX)")}</span><span class="chart-footnote">${chartMode === 'history' ? t("Bounded persistent history") : t('{count} samples · RAM buffer', {count: state.points.length})}</span></div></section>
  <section class="panel clients-panel"><div class="panel-head"><div><h2>${t("Clients")} <span class="count-chip">${state.peers.length}</span></h2><p>${t("Devices on your network and their status")}</p></div></div>${peerTable(state.peers.slice(0, 5), true)}<div class="table-note">${icon('info')}${t("Activity is estimated from the latest handshake.")}</div></section></div>
  <aside class="dashboard-secondary"><section class="panel health-panel"><div class="panel-head"><h2>${t("Server health")}</h2><span class="badge ${state.interfaceUp ? 'green' : 'amber'}"><i></i>${state.interfaceUp ? t("Running") : t("Needs attention")}</span></div><div class="server-identity"><span class="server-icon">${icon('server')}</span><div><strong>${esc(state.server.name)}</strong><span>${esc(state.server.interface)} <b>·</b> ${esc(state.server.address)}</span></div></div><div class="resource"><div><span>${t("CPU")}</span><strong>${state.mode === 'demo' ? '—' : `${state.resources.hostCPU.toFixed(1)}%`}</strong></div><progress max="100" value="${state.mode === 'demo' ? 0 : state.resources.hostCPU}"></progress><small>${state.mode === 'demo' ? t("Measured on the Linux server") : t("Server-wide usage")}</small></div><div class="resource"><div><span>${t("System memory")}</span><strong>${state.resources.hostMemoryTotal ? `${Math.round(state.resources.hostMemoryUsed / state.resources.hostMemoryTotal * 100)}%` : '—'}</strong></div><progress max="${state.resources.hostMemoryTotal || 1}" value="${state.resources.hostMemoryUsed}"></progress><small>${state.resources.hostMemoryTotal ? `${bytes(state.resources.hostMemoryUsed)} / ${bytes(state.resources.hostMemoryTotal)}` : t("Measured on the Linux server")}</small></div><div class="health-meta"><div><span>${t("Application data")}</span><strong>${bytes(state.resources.controlBytes + state.resources.telemetryBytes)}</strong></div><div><span>${t("Interface")}</span><strong class="mono">${esc(state.server.interface)} : ${state.server.port}</strong></div></div></section>
    <section class="panel recent-panel"><div class="panel-head"><h2>${t("Recent activity")}</h2></div>${eventList(state.events.slice(-3).reverse(), true)}</section></aside></div>`;
}

function graph(points: Point[]): string {
  if (points.length < 2) return `<div class="chart-wait">${icon('pulse')}<strong>${chartMode === 'history' ? t("No history samples yet") : t("Collecting first samples")}</strong><span>${chartMode === 'history' ? t("Samples will appear at your selected recording interval.") : t("The live traffic chart will appear in a few seconds.")}</span></div>`;
  const w = 760, h = 210, left = 0, max = Math.max(10000, ...points.flatMap(p => [p.rx, p.tx])) * 1.25;
  const make = (key: 'rx' | 'tx'): string => points.map((p, i) => `${i === 0 ? 'M' : 'L'}${(left + i / (points.length - 1) * w).toFixed(1)},${(h - p[key] / max * (h - 8)).toFixed(1)}`).join(' ');
  const rx = make('rx'), tx = make('tx');
  const labels = [0, 1, 2, 3, 4].map(i => points[Math.round((points.length - 1) * i / 4)]);
  return `<div class="chart"><div class="chart-y">${[1, .75, .5, .25, 0].map(n => `<span>${rate(max * n)}</span>`).join('')}</div><div class="chart-drawing"><svg viewBox="0 0 ${w} ${h}" preserveAspectRatio="none" role="img" aria-label="${t("Received and sent traffic chart")}"><defs><linearGradient id="rx-fill" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stop-color="#4e8b66" stop-opacity=".2"/><stop offset="100%" stop-color="#4e8b66" stop-opacity=".01"/></linearGradient></defs>${[0, .25, .5, .75, 1].map(n => `<line x1="0" y1="${n * h}" x2="${w}" y2="${n * h}" stroke="#e9ede8" stroke-dasharray="4 5"/>`).join('')}<path d="${rx} L${w},${h} L0,${h} Z" fill="url(#rx-fill)"/><path d="${tx} L${w},${h} L0,${h} Z" fill="#8b9dab" fill-opacity=".04"/><path d="${rx}" fill="none" stroke="#4c8866" stroke-width="2.5" vector-effect="non-scaling-stroke" stroke-linejoin="round"/><path d="${tx}" fill="none" stroke="#64788e" stroke-dasharray="6 4" stroke-width="2" vector-effect="non-scaling-stroke" stroke-linejoin="round"/></svg><div class="chart-x">${labels.map(p => `<span>${time(p.time)}</span>`).join('')}</div></div></div>`;
}
function peerTable(peers: Peer[], compact = false): string {
  if (!peers.length) return empty('devices', query ? t("No matching clients") : t("Create your first connection"), query ? t("Try a different name or IP address.") : t("Add a client, download its configuration, and connect."), !query ? addButton() : '');
  return `<div class="table-scroll"><table class="peer-table"><thead><tr><th>${t("CLIENT")}</th><th>${t("STATUS")}</th><th>${t("TRANSFER")}</th><th>${t("LAST HANDSHAKE")}</th><th class="align-right">${t("ACTIONS")}</th></tr></thead><tbody>${peers.map((p, i) => `<tr><td><div class="peer-name"><span class="device-avatar color-${i % 4}">${icon(/phone|pixel|iphone/i.test(p.name) ? 'devices' : /sunucu|server/i.test(p.name) ? 'server' : 'devices')}</span><div><strong>${esc(p.name)}</strong><span class="mono">${esc(p.address)}</span></div></div></td><td>${status(p)}</td><td><div class="transfer"><span>↓ ${bytes(p.rxBytes)}</span><small>↑ ${bytes(p.txBytes)}</small></div></td><td><span class="handshake">${ago(p.lastHandshake)}</span>${!compact && p.expiresAt ? `<small class="expires">${t('Expires: {date}', {date: date(p.expiresAt)})}</small>` : ''}</td><td><div class="row-actions"><button class="icon-btn" data-qr="${esc(p.id)}" aria-label="${esc(t('QR code for {name}', {name: p.name}))}" title="${t("QR code")}" ${!p.hasConfig ? 'disabled' : ''}>${icon('qr')}</button><button class="icon-btn" data-download="${esc(p.id)}" aria-label="${esc(t('Download configuration for {name}', {name: p.name}))}" title="${t("Download configuration")}" ${!p.hasConfig ? 'disabled' : ''}>${icon('down')}</button><button class="icon-btn" data-detail="${esc(p.id)}" aria-label="${esc(t('Details for {name}', {name: p.name}))}" title="${t("Client details")}">${icon('dots')}</button></div></td></tr>`).join('')}</tbody></table></div>`;
}
function peersPage(): string {
  const peers = state.peers.filter(p => (peerFilter === 'all' || p.status === peerFilter) && `${p.name} ${p.address} ${p.endpoint}`.toLocaleLowerCase(locale()).includes(query.toLocaleLowerCase(locale())));
  return `${pageHead(t("CONNECTIONS"), t("Clients"), t("Add devices, manage access, and monitor connection status."), `<button class="btn secondary" data-action="import">${icon('upload')}${t("Import")}</button>${addButton()}`)}<div class="peer-summary"><span><i class="tiny-dot green-dot"></i><strong>${state.peers.filter(p => p.status === 'active').length}</strong> ${t("Active")}</span><span><i class="tiny-dot amber-dot"></i><strong>${state.peers.filter(p => p.status === 'idle').length}</strong> ${t("Idle")}</span><span><i class="tiny-dot gray-dot"></i><strong>${state.peers.filter(p => !p.enabled).length}</strong> ${t("Disabled")}</span></div><section class="panel"><div class="table-toolbar"><div class="filter-tabs">${[['all', t("All")], ['active', t("Active")], ['idle', t("Idle")], ['disabled', t("Disabled")]].map(([key, label]) => `<button class="${peerFilter === key ? 'selected' : ''}" data-filter="${key}">${label}${key === 'all' ? `<span>${state.peers.length}</span>` : ''}</button>`).join('')}</div><label class="search-box">${icon('search')}<input id="peer-search" placeholder="${t("Search name or IP address…")}" value="${esc(query)}" aria-label="${t("Search clients")}"/><kbd>/</kbd></label></div><div id="peer-table-wrap">${peerTable(peers)}</div><div class="table-note">${icon('info')}${t("Handshake activity is an estimate. Counters can reset when the interface is recreated.")}</div></section>`;
}
function eventList(events: Event[], compact = false): string {
  if (!events.length) return empty('pulse', t("No activity yet"), t("Administrative actions will appear here."));
  return `<div class="event-list ${compact ? 'compact' : ''}">${events.map(e => `<div class="event-row"><span class="event-symbol ${e.level}">${icon(e.level === 'warning' || e.level === 'error' ? 'info' : e.action.startsWith('peer') ? 'devices' : 'check')}</span><div><strong>${esc(serverMessage(e.detail))}</strong><span>${compact ? ago(e.time) : `${date(e.time)} · ${time(e.time)} · ${esc(e.action)}`}</span></div>${!compact ? `<span class="badge ${e.level === 'warning' ? 'amber' : e.level === 'error' ? 'red' : 'muted'}">${e.level === 'warning' ? t("Warning") : e.level === 'error' ? t("Error") : t("Info")}</span>` : ''}</div>`).join('')}</div>`;
}
function activityPage(): string { return `${pageHead(t("OBSERVABILITY"), t("Activity"), t("Client actions, administrator logins, and system events."), `<button class="btn secondary" data-action="export-events">${icon('down')}${t("Export activity")}</button>`)}<section class="panel"><div class="panel-head"><h2>${t("Administrative events")} <span class="count-chip">${state.events.length}</span></h2><span class="badge subtle"><i class="live-dot"></i>${t("LIVE FEED")}</span></div>${eventList([...state.events].reverse())}</section>`; }
function toggle(name: string, checked: boolean, label: string, text: string, tag = ''): string { return `<label class="toggle-row"><div class="toggle-description"><strong>${label}${tag ? `<span class="mini-label">${tag}</span>` : ''}</strong><p>${text}</p></div><span class="switch"><input type="checkbox" name="${name}" ${checked ? 'checked' : ''}/><span class="switch-track"></span></span></label>`; }
function storagePage(): string {
  const cfg = state.settings, r = state.resources;
  return `${pageHead(t("RESOURCE MANAGEMENT"), t("Storage & monitoring"), t("More visibility when you need it. You set the limits."))}<div class="settings-layout"><form id="storage-form" class="settings-primary"><section class="panel"><div class="panel-head"><div><h2>${t("Monitoring preferences")}</h2><p>${t("Choose what to store for network traffic and events.")}</p></div><span class="badge green">${icon('leaf')}${t("LIGHTWEIGHT DEFAULTS")}</span></div><div class="toggle-rows"><div class="always-on"><div><strong>${t("Live monitoring")}</strong><p>${t("Recent samples stay in RAM. No persistent traffic data is written.")}</p></div><span class="badge green"><i></i>${t("Always on")}</span></div>${toggle('historyEnabled', cfg.historyEnabled, t("Traffic history"), t("Record traffic totals and client counters at intervals."), t("OPTIONAL"))}${toggle('auditEnabled', cfg.auditEnabled, t("Administrative history"), t("Store client actions, login attempts, and setting changes on disk."))}${toggle('metricsEnabled', cfg.metricsEnabled, t("Prometheus metrics"), t("Enable token-protected /metrics. No separate monitoring service is installed."))}</div></section><section class="panel"><div class="panel-head"><div><h2>${t("Retention limits")}</h2><p>${t("This budget applies when persistent recording is enabled.")}</p></div>${icon('storage')}</div><div class="form-grid storage-fields"><label>${t("Retention period")}<div class="input-unit"><input name="retentionDays" type="number" min="1" max="30" required value="${cfg.retentionDays}"/><span>${t("days")}</span></div><small>${t("1–30 days; older records are removed.")}</small></label><label>${t("Total history budget")}<div class="input-unit"><input name="budgetMB" type="number" min="16" max="512" required value="${cfg.budgetMB}"/><span>MB</span></div><small>${t("Includes the history database and transaction journal.")}</small></label><label>${t("Sampling interval")}<select name="sampleSeconds">${[30, 60, 120, 300, 600, 3600].map(n => `<option value="${n}" ${cfg.sampleSeconds === n ? 'selected' : ''}>${n < 60 ? t('{count} seconds', {count: n}) : t('{count} minutes', {count: n / 60})}</option>`).join('')}</select><small>${t("Less frequent sampling means fewer disk writes.")}</small></label></div><div class="policy-note">${icon('shield')}${t("History recording pauses below 64 MB free space. Existing VPN connections continue.")}</div></section><div class="form-actions"><span>${t("Changes take effect when saved.")}</span><button class="btn primary" type="submit">${icon('check')}${t("Save preferences")}</button></div></form><aside class="settings-secondary"><section class="budget-card"><span class="eyebrow">${t("STORAGE SUMMARY")}</span><h2>${bytes(r.telemetryBytes)}<span>/ ${cfg.budgetMB} MB</span></h2><progress value="${r.telemetryBytes}" max="${cfg.budgetMB * 1024 ** 2}"></progress><div class="budget-status"><i class="tiny-dot ${state.historyPaused ? 'amber-dot' : 'green-dot'}"></i>${state.historyPaused ? t("Recording paused") : cfg.historyEnabled || cfg.auditEnabled ? t("Selected recording enabled") : t("Persistent recording off")}</div><dl><div><dt>${t("Configuration data")}</dt><dd>${bytes(r.controlBytes)}</dd></div><div><dt>${t("History + journal files")}</dt><dd>${bytes(r.telemetryBytes)}</dd></div><div><dt>${t("Free disk space")}</dt><dd>${state.mode === 'demo' ? '—' : bytes(r.diskFree)}</dd></div></dl><p>${t("Configuration is separate from history. Disabling recording keeps existing records until expiry or cleanup.")}</p></section><section class="panel cleanup-card"><h3>${t("Clear history")}</h3><p>${t("Removes traffic and persistent event records. Clients and server settings are kept.")}</p><button class="btn danger-outline full" type="button" data-action="clear-history">${icon('trash')}${t("Clear persistent history")}</button></section></aside></div>`;
}
function field(label: string, name: string, value: string | number, help = '', extra = ''): string { return `<label>${label}<input name="${name}" value="${esc(value)}" ${extra}/>${help ? `<small>${help}</small>` : ''}</label>`; }
function panelAccessForm(): string {
  return `<form id="panel-access-form" class="panel"><div class="panel-head"><div><h2>${t('Panel access')}</h2><p>${t('Control who can reach the management panel. VPN clients are unaffected.')}</p></div>${icon('shield')}</div><div class="form-grid"><label>${t('Access mode')}<select name="mode"><option value="public">${t('Public access')}</option><option value="local">${t('This server only')}</option><option value="lan">${t('Private networks only')}</option></select><small>${t('Private networks include VPN and LAN addresses; this is based on source IP, not location.')}</small></label><label>${t('IP allowlist')}<textarea name="allowlist" rows="4" maxlength="4096" placeholder="203.0.113.10&#10;10.0.0.0/8"></textarea><small>${t('Optional IPs or CIDRs, separated by commas or lines. Loopback is always allowed. Forwarded headers are ignored.')}</small></label></div><div class="policy-note">${t('HTTP is unencrypted. Use HTTPS on untrusted networks.')}</div><div class="panel-form-footer"><button class="btn primary" type="submit" disabled>${t('Save access')}</button></div></form>`;
}
function settingsPage(): string {
  const s = state.server;
  return `${pageHead(t("CONFIGURATION"), t("Server settings"), t("Manage connection settings and administrator access."))}<div class="settings-layout"><div class="settings-primary"><form id="server-form" class="panel"><div class="panel-head"><div><h2>${t("WireGuard server")}</h2><p>${t("Configuration changes apply to the selected interface.")}</p></div><span class="badge muted mono">${esc(s.interface)}</span></div><div class="form-grid">${field(t("Server name"), 'name', s.name, '', 'required maxlength="64"')}${field('Endpoint', 'endpoint', s.endpoint, t("The public IP address or domain clients connect to."), 'placeholder="vpn.example.com"')}${field(t("VPN address"), 'address', s.address, t("Example: 10.64.0.1/24"), 'required')}${field(t("UDP port"), 'port', s.port, '', 'type="number" min="1" max="65535" required')}${field(t("DNS servers"), 'dns', s.dns, t("Comma-separated IP addresses."))}${field(t("Client Allowed IPs"), 'allowedIPs', s.allowedIPs, t("Example: 0.0.0.0/0 or private subnets."), 'required')}${field('MTU', 'mtu', s.mtu, t("Changing the address/MTU of a running interface requires maintenance."), 'type="number" min="1280" max="1500" required')}</div><div class="panel-form-footer"><button class="btn secondary" type="button" id="use-server-dns">${t("Use server DNS")}</button><button class="btn primary" type="submit">${icon('check')}${t("Apply server settings")}</button></div></form><section class="panel import-card"><span class="server-icon">${icon('upload')}</span><div><h3>${t("Have an existing WireGuard configuration?")}</h3><p>${t("Import interface and clients from a server .conf file. Executable hooks are rejected.")}</p></div><button class="btn secondary" data-action="import">${t("Import")}</button></section><form id="password-form" class="panel"><div class="panel-head"><div><h2>${t("Administrator password")}</h2><p>${state.mode === 'demo' ? t("Available on a live Linux installation.") : t("Changing the password signs out all administrator sessions.")}</p></div>${icon('lock')}</div><div class="form-grid"><label>${t("Current password")}<input name="current" type="password" autocomplete="current-password" required ${state.mode === 'demo' ? 'disabled' : ''}/></label><label>${t("New password")}<input name="password" type="password" autocomplete="new-password" minlength="12" maxlength="256" required ${state.mode === 'demo' ? 'disabled' : ''}/><small>${t("At least 12 characters.")}</small></label></div><div class="panel-form-footer"><button class="btn secondary" ${state.mode === 'demo' ? 'disabled' : ''}>${t("Change password")}</button></div></form></div><aside class="settings-secondary"><section class="panel server-info-card"><span class="server-icon">${icon('shield')}</span><h3>${t("An independent management layer")}</h3><p>${t("Linux handles WireGuard traffic. Existing VPN connections stay up when the panel stops.")}</p><dl><div><dt>${t("Interface")}</dt><dd class="mono">${esc(s.interface)}</dd></div><div><dt>${t("Status")}</dt><dd>${state.interfaceUp ? t("Running") : t("Not ready")}</dd></div><div><dt>${t("Version")}</dt><dd>${esc(state.version)}</dd></div><div><dt>${t("Uptime")}</dt><dd>${duration(state.startedAt)}</dd></div></dl><label class="key-label">${t("SERVER PUBLIC KEY")}</label><code>${esc(s.publicKey)}</code><button class="text-btn" data-action="copy-key">${icon('copy')}${t("Copy public key")}</button></section>${state.mode !== 'demo' ? `<button class="btn secondary full" data-action="logout">${icon('logout')}${t("Sign out")}</button>` : ''}</aside></div>`;
}

function bindMain(): void {
  if (view === 'settings') {
    document.querySelector('.settings-primary')?.insertAdjacentHTML('afterbegin', panelAccessForm());
    const form = document.querySelector<HTMLFormElement>('#panel-access-form')!;
    form.addEventListener('input', () => { form.dataset.restored = 'true'; });
    form.addEventListener('change', () => { form.dataset.restored = 'true'; });
    void api<{mode: string; allowlist: string[]}>('/api/panel-access').then(p => {
      if (!form.isConnected) return;
      if (form.dataset.restored !== 'true') {
      (form.elements.namedItem('mode') as HTMLSelectElement).value = p.mode;
      (form.elements.namedItem('allowlist') as HTMLTextAreaElement).value = (p.allowlist ?? []).join('\n');
      }
      form.querySelector<HTMLButtonElement>('button[type="submit"]')!.disabled = false;
    }).catch(e => toast(String(e)));
    form.addEventListener('submit', event => {
      event.preventDefault();
      const f = new FormData(form);
      const policy = { mode: String(f.get('mode')), allowlist: String(f.get('allowlist')).split(/[\s,]+/).filter(Boolean) };
      confirmDialog(t('Save panel access?'), t('Access changes may disconnect you. Keep SSH access available for recovery.'), t('Save access'), async () => {
        await api('/api/panel-access', 'PUT', policy);
        closeDialog(); toast(t('Panel access saved'));
      });
    });
  }
  document.querySelectorAll<HTMLProgressElement>('.resource progress').forEach(progress => progress.setAttribute('aria-label', progress.closest('.resource')?.querySelector('div span')?.textContent ?? t("Server resource")));
  document.querySelector('.budget-card progress')?.setAttribute('aria-label',t("History storage budget usage"));
  if (view === 'storage' && state.settings.metricsEnabled) {
    document.querySelector('#storage-form > .panel')?.insertAdjacentHTML('afterend', `<section class="panel"><div class="panel-head"><div><h2>${t("Metrics access")}</h2><p>${t("Create a separate, read-only access token for Prometheus.")}</p></div>${icon('pulse')}</div><div class="metrics-info"><code>/metrics</code><p>${t("Add the Bearer token to your monitoring system. It can only read metrics.")}</p><button class="btn secondary" type="button" id="create-metrics-token">${t("Create / rotate access token")}</button></div></section>`);
    document.querySelector('#create-metrics-token')?.addEventListener('click', () => action(async () => {
      const result = await api<{ token: string }>('/api/metrics-token', 'POST');
      openDialog(t("Metrics access token"), `<p class="modal-subtitle">${t("Shown only once. Creating a new token invalidates the previous token.")}</p><code class="public-key">${esc(result.token)}</code><div class="modal-note metrics-note">${icon('lock')}${t("Read-only metrics access. Cannot perform management actions.")}</div><div class="modal-actions"><button class="btn primary" id="copy-metrics-token">${icon('copy')}${t("Copy token")}</button></div>`);
      modal.querySelector('#copy-metrics-token')!.addEventListener('click', () => action(async () => { await navigator.clipboard.writeText(result.token); toast(t("Metrics token copied")); }));
    }));
  }
  document.querySelectorAll<HTMLButtonElement>('[data-action="add"]').forEach(el => el.addEventListener('click', createDialog));
  document.querySelectorAll<HTMLButtonElement>('[data-chart]').forEach(el => el.addEventListener('click', () => action(async () => { chartMode = el.dataset.chart!; if (chartMode === 'history' && state.settings.historyEnabled) history = await api<Point[]>('/api/history'); renderMain(); })));
  document.querySelectorAll<HTMLButtonElement>('[data-qr]').forEach(el => el.addEventListener('click', () => action(() => qrDialog(el.dataset.qr!))));
  document.querySelectorAll<HTMLButtonElement>('[data-download]').forEach(el => el.addEventListener('click', () => action(() => downloadConfig(el.dataset.download!))));
  document.querySelectorAll<HTMLButtonElement>('[data-detail]').forEach(el => el.addEventListener('click', () => detailDialog(el.dataset.detail!)));
  document.querySelectorAll<HTMLButtonElement>('[data-filter]').forEach(el => el.addEventListener('click', () => { peerFilter = el.dataset.filter!; renderMain(); }));
  document.querySelector<HTMLInputElement>('#peer-search')?.addEventListener('input', event => {
    query = (event.target as HTMLInputElement).value;
    const peers = state.peers.filter(p => (peerFilter === 'all' || p.status === peerFilter) && `${p.name} ${p.address} ${p.endpoint}`.toLocaleLowerCase(locale()).includes(query.toLocaleLowerCase(locale())));
    document.querySelector('#peer-table-wrap')!.innerHTML = peerTable(peers);
    document.querySelectorAll<HTMLButtonElement>('#peer-table-wrap [data-qr]').forEach(el => el.addEventListener('click', () => action(() => qrDialog(el.dataset.qr!))));
    document.querySelectorAll<HTMLButtonElement>('#peer-table-wrap [data-download]').forEach(el => el.addEventListener('click', () => action(() => downloadConfig(el.dataset.download!))));
    document.querySelectorAll<HTMLButtonElement>('#peer-table-wrap [data-detail]').forEach(el => el.addEventListener('click', () => detailDialog(el.dataset.detail!)));
    document.querySelectorAll<HTMLButtonElement>('#peer-table-wrap [data-action="add"]').forEach(el => el.addEventListener('click', createDialog));
  });
  document.querySelectorAll<HTMLButtonElement>('[data-action="import"]').forEach(el => el.addEventListener('click', importDialog));
  document.querySelector('[data-action="clear-history"]')?.addEventListener('click', () => confirmDialog(t("Clear persistent history?"), t("Traffic and persistent administrative records will be deleted. Clients and settings are kept."), t("Clear history"), async () => { await api('/api/history', 'DELETE'); closeDialog(); await refresh(true); navigate('storage'); toast(t("Persistent history cleared")); }));
  document.querySelector('[data-action="export-events"]')?.addEventListener('click', () => { download(new Blob([JSON.stringify(state.events, null, 2)], { type: 'application/json' }), `wgs-activity-${new Date().toISOString().slice(0, 10)}.json`); toast(t("Visible activity exported")); });
  document.querySelector('[data-action="copy-key"]')?.addEventListener('click', () => action(async () => { await navigator.clipboard.writeText(state.server.publicKey); toast(t("Public key copied")); }));
  document.querySelector('[data-action="logout"]')?.addEventListener('click', () => action(async () => { await api('/api/logout', 'POST'); clearInterval(poll); session.authenticated = false; renderAuth(); }));
  document.querySelector<HTMLFormElement>('#storage-form')?.addEventListener('submit', event => { event.preventDefault(); const form = event.currentTarget as HTMLFormElement; void submitForm(form, async () => { const f = new FormData(form); await api('/api/settings', 'PUT', { historyEnabled: f.has('historyEnabled'), auditEnabled: f.has('auditEnabled'), metricsEnabled: f.has('metricsEnabled'), retentionDays: Number(f.get('retentionDays')), budgetMB: Number(f.get('budgetMB')), sampleSeconds: Number(f.get('sampleSeconds')) }); await refresh(true); renderMain(); toast(t("Monitoring preferences saved")); }); });
  document.querySelector('#use-server-dns')?.addEventListener('click', () => action(async () => {
    const defaults = await api<{ dns: string }>('/api/network-defaults');
    if (!defaults.dns) throw new Error(t("Server DNS could not be detected. Enter DNS addresses manually."));
    const input = document.querySelector<HTMLInputElement>('#server-form [name="dns"]');
    if (input) input.value = defaults.dns;
  }));
  document.querySelector<HTMLFormElement>('#server-form')?.addEventListener('submit', event => { event.preventDefault(); const form = event.currentTarget as HTMLFormElement; void submitForm(form, async () => { const f = new FormData(form); await api('/api/server', 'PUT', { ...state.server, name: String(f.get('name')), endpoint: String(f.get('endpoint')).trim(), address: String(f.get('address')).trim(), port: Number(f.get('port')), dns: String(f.get('dns')), allowedIPs: String(f.get('allowedIPs')), mtu: Number(f.get('mtu')) }); await refresh(true); toast(state.mode === 'demo' ? t("Demo server settings updated") : t("Server settings applied")); }); });
  document.querySelector<HTMLFormElement>('#password-form')?.addEventListener('submit', event => { event.preventDefault(); const form = event.currentTarget as HTMLFormElement; void submitForm(form, async () => { const f = new FormData(form); await api('/api/password', 'POST', { current: f.get('current'), password: f.get('password') }); clearInterval(poll); session.authenticated = false; renderAuth(); toast(t("Password changed. Sign in again.")); }); });
}
async function submitForm(form: HTMLFormElement, fn: () => Promise<void>): Promise<void> {
  if (form.getAttribute('aria-busy') === 'true') return;
  const button = form.querySelector<HTMLButtonElement>('button[type="submit"]') ?? form.querySelector<HTMLButtonElement>('button:last-child');
  form.querySelector('.form-feedback')?.remove();
  form.setAttribute('aria-busy', 'true');
  if (button) { button.disabled = true; button.setAttribute('aria-busy', 'true'); }
  try { await fn(); }
  catch (err) {
    const message = err instanceof Error ? err.message : t('Action failed');
    if (form.isConnected) {
      const feedback = document.createElement('p');
      feedback.className = 'form-feedback'; feedback.setAttribute('role', 'alert');
      feedback.textContent = message;
      form.addEventListener('input', () => feedback.remove(), { once: true });
      const body = form.querySelector('.client-form-body') ?? form;
      body.append(feedback); feedback.scrollIntoView({ block: 'nearest', behavior: 'instant' });
    }
    toast(message, true);
  } finally {
    form.removeAttribute('aria-busy');
    if (button) { button.disabled = false; button.removeAttribute('aria-busy'); }
  }
}
function openDialog(title: string, content: string, cls = ''): void {
  if (modal.open) modal.close(); modal.className = cls; modal.innerHTML = `<div class="modal-head"><div><div class="eyebrow">WIREGUARD SIMPLE</div><h2 id="modal-title">${title}</h2></div><button class="icon-btn" id="close-modal" aria-label="${t("Close dialog")}">${icon('close')}</button></div>${content}`;
  modal.querySelector('#close-modal')!.addEventListener('click', closeDialog); modal.showModal();
}
function closeDialog(): void { modal.close(); if (qrBlob) { URL.revokeObjectURL(qrBlob); qrBlob = ''; } modal.innerHTML = ''; }
modal.addEventListener('cancel', event => { event.preventDefault(); closeDialog(); });
modal.addEventListener('click', event => { if (event.target === modal) { const box = modal.getBoundingClientRect(); if (event.clientX < box.left || event.clientX > box.right || event.clientY < box.top || event.clientY > box.bottom) closeDialog(); } });
function createDialog(): void {
  const s = state.server;
  const addressPreview = clientAddressPreview(s.address, state.peers);
  const optional = (label: string, name: string, placeholder = '', extra = '') => field(label, name, '', '', `placeholder="${esc(placeholder)}" ${extra}`);
  openDialog(t("New client"), `<form id="create-form"><div class="client-form-body"><p class="modal-subtitle">${t("Name your device. We will generate its keys and allocate an available IP address.")}</p>
    <div class="form-grid single"><label>${t("Client name")}<input name="name" placeholder="${t("e.g. Work MacBook")}" required maxlength="64" autocomplete="off" autofocus/></label><label>${t("Client VPN address")}<input name="address" value="${esc(addressPreview.next)}" placeholder="${t("Generate automatically")}" autocomplete="off" spellcheck="false" aria-describedby="client-subnet client-address-feedback"/><small id="client-subnet">${esc(t("VPN subnet: {subnet}", { subnet: addressPreview.subnet || s.address }))}</small><small id="client-address-feedback" aria-live="polite"></small></label><label>${t("Access expiry date")} <span class="optional">${t("Optional")}</span><input name="expires" type="datetime-local" min="${new Date(Date.now() + 60000).toLocaleString('sv-SE').slice(0, 16).replace(' ', 'T')}"/><small>${t("Leave empty for no time limit.")}</small></label></div>
    <details id="client-advanced" class="client-advanced-settings"><summary>${t("Advanced settings")}</summary><p>${t("Leave fields empty to use the defaults. Changes apply only to this client.")}</p><div class="form-grid">
      <label>${t("DNS servers")}<select name="dnsMode"><option value="inherit">${t("Use server defaults")}</option><option value="custom">${t("Custom DNS")}</option><option value="device">${t("Keep device DNS")}</option></select><input name="dns" disabled placeholder="${esc(s.dns || t('Comma-separated IP addresses.'))}" autocomplete="off"/></label>
      ${optional(t("Client Allowed IPs"), 'allowedIPs', s.allowedIPs)}
      ${optional('Endpoint', 'endpoint', s.endpoint || 'SERVER_IP')}
      ${optional(t("UDP port"), 'port', String(s.port), 'type="number" min="1" max="65535"')}
      ${optional('MTU', 'mtu', String(s.mtu), 'type="number" min="1280" max="1500"')}
      ${optional(t("Persistent keepalive (seconds)"), 'persistentKeepalive', '25', 'type="number" min="0" max="65535"')}
      ${optional(t("Client listen port"), 'listenPort', '0', 'type="number" min="0" max="65535"')}
      ${field('FwMark', 'fwMark', '', t("Linux policy routing. Leave empty unless required."), 'placeholder="0" type="number" min="0" max="4294967295"')}
      ${optional(t("Client private key"), 'privateKey', t("Generate automatically"), 'type="password" autocomplete="new-password" spellcheck="false"')}
      ${field(t("Preshared key"), 'presharedKey', '', t("A unique key is generated automatically when left empty."), `placeholder="${esc(t("Generate automatically"))}" type="password" autocomplete="new-password" spellcheck="false"`)}
    </div><small>${t("0 disables keepalive. Listen port 0 is automatic. Routing settings do not enforce access restrictions.")}</small></details>
    <div class="modal-note">${icon('shield')}${t("Each client gets an independent key pair.")}</div></div><div class="modal-actions"><button class="btn secondary" type="button" id="cancel-create">${t("Cancel")}</button><button class="btn primary" type="submit">${icon('plus')}${t("Create client")}</button></div></form>`, 'create-client');
  modal.querySelector('#cancel-create')!.addEventListener('click', closeDialog);
  const address = modal.querySelector<HTMLInputElement>('[name="address"]')!;
  const feedback = modal.querySelector<HTMLElement>('#client-address-feedback')!;
  const validateAddress = () => {
    const currentPreview = clientAddressPreview(state.server.address, state.peers);
    modal.querySelector('#client-subnet')!.textContent = t('VPN subnet: {subnet}', { subnet: currentPreview.subnet || state.server.address });
    const issue = clientAddressIssue(state.server.address, state.peers, address.value);
    address.setCustomValidity(issue ? t(issue) : '');
    address.setAttribute('aria-invalid', String(Boolean(issue)));
    feedback.classList.toggle('field-error', Boolean(issue));
    feedback.textContent = issue ? t(issue) : address.value.trim()
      ? t("This address stays assigned to this client.")
      : t("Leave empty to assign an available address automatically.");
    return !issue;
  };
  address.addEventListener('input', validateAddress);
  address.addEventListener('blur', () => { address.value = normalizeClientAddress(address.value); validateAddress(); });
  validateAddress();
  const advanced = modal.querySelector<HTMLDetailsElement>('#client-advanced')!;
  advanced.addEventListener('toggle', () => modal.classList.toggle('client-advanced', advanced.open));
  modal.querySelector<HTMLSelectElement>('[name="dnsMode"]')!.addEventListener('change', event => {
    const custom = (event.currentTarget as HTMLSelectElement).value === 'custom';
    const dns = modal.querySelector<HTMLInputElement>('[name="dns"]')!;
    dns.disabled = !custom; dns.required = custom;
  });
  modal.querySelector<HTMLFormElement>('#create-form')!.addEventListener('submit', event => {
    event.preventDefault(); const form = event.currentTarget as HTMLFormElement;
    void submitForm(form, async () => {
      // Refresh occupied addresses before submission; the server still performs
      // the final atomic check if another administrator claims one concurrently.
      await refresh(true);
      if (!form.isConnected || !modal.open) return;
      if (!validateAddress()) { address.reportValidity(); return; }
      address.value = normalizeClientAddress(address.value);
      const f = new FormData(form); const expires = String(f.get('expires'));
      const options: Record<string, string | number> = {};
      for (const key of ['address', 'allowedIPs', 'endpoint', 'privateKey', 'presharedKey']) { const value = String(f.get(key) ?? '').trim(); if (value) options[key] = value; }
      for (const key of ['port', 'mtu', 'persistentKeepalive', 'listenPort', 'fwMark']) { const value = String(f.get(key) ?? '').trim(); if (value) options[key] = Number(value); }
      if (f.get('dnsMode') === 'custom') options.dns = String(f.get('dns') ?? '').trim();
      if (f.get('dnsMode') === 'device') options.dns = '';
      let peer: Peer;
      try {
        peer = await api<Peer>('/api/peers', 'POST', { name: f.get('name'), expiresAt: expires ? new Date(expires).toISOString() : null, options });
      } catch (error) {
        // Keep the form and typed address intact on a concurrent conflict.
        if (error instanceof Error && error.message === t('Client address is already in use')) {
          address.setCustomValidity(error.message); address.setAttribute('aria-invalid', 'true');
          feedback.textContent = error.message; feedback.classList.add('field-error'); address.focus();
          return;
        }
        throw error;
      }
      closeDialog(); await refresh(true); toast(t("Client created")); detailDialog(peer.id);
    });
  });
}
function detailDialog(id: string): void {
  const p = state.peers.find(p => p.id === id); if (!p) return;
  openDialog(esc(p.name), `<div class="detail-status">${status(p)}<span class="mono">${esc(p.address)}</span></div><dl class="peer-details"><div><dt>${t("Last handshake")}</dt><dd>${ago(p.lastHandshake)}</dd></div><div><dt>${t("Received / sent")}</dt><dd>${bytes(p.rxBytes)} / ${bytes(p.txBytes)}</dd></div><div><dt>Endpoint</dt><dd class="mono">${esc(p.endpoint || t("Not known yet"))}</dd></div><div><dt>${t("Access expiry")}</dt><dd>${p.expiresAt ? new Date(p.expiresAt).toLocaleString(locale()) : t("Unlimited")}</dd></div></dl><label class="key-label">${t("CLIENT PUBLIC KEY")}</label><code class="public-key">${esc(p.publicKey)}</code>${!p.hasConfig ? `<p class="modal-subtitle">${t("This client was imported without its private key. QR codes and configurations cannot be generated.")}</p>` : ''}<div class="detail-downloads"><button class="btn secondary" id="detail-qr" ${!p.hasConfig ? 'disabled' : ''}>${icon('qr')}${t("QR code")}</button><button class="btn primary" id="detail-download" ${!p.hasConfig ? 'disabled' : ''}>${icon('down')}${t("Configuration")}</button></div><div class="detail-danger"><button class="text-btn" id="toggle-peer">${icon(p.enabled ? 'pause' : 'play')}${p.enabled ? t("Pause access") : t("Enable access")}</button><button class="text-btn red-text" id="delete-peer">${icon('trash')}${t("Delete client")}</button></div>`);
  modal.querySelector('#detail-qr')!.addEventListener('click', () => action(() => qrDialog(id)));
  modal.querySelector('#detail-download')!.addEventListener('click', () => action(() => downloadConfig(id)));
  modal.querySelector('#toggle-peer')!.addEventListener('click', () => action(async () => { await api(`/api/peers/${id}`, 'PATCH', { enabled: !p.enabled }); closeDialog(); await refresh(true); toast(p.enabled ? t("Client access paused") : t("Client enabled")); }));
  modal.querySelector('#delete-peer')!.addEventListener('click', () => confirmDialog(t("Delete client?"), esc(t('Delete VPN access and the stored key for {name} from this server.', {name: p.name})), t("Delete client"), async () => { await api(`/api/peers/${id}`, 'DELETE'); closeDialog(); await refresh(true); toast(t("Client deleted")); }));
}
async function qrDialog(id: string): Promise<void> {
  const p = state.peers.find(p => p.id === id); if (!p) return;
  const response = await fetch(`/api/peers/${id}/qr`, { credentials: 'same-origin' }); if (!response.ok) { const error = await response.json(); throw new Error(serverMessage(error.error)); }
  if (qrBlob) URL.revokeObjectURL(qrBlob); qrBlob = URL.createObjectURL(await response.blob());
  openDialog(t("Connect your device"), `<p class="modal-subtitle">${esc(t('Open WireGuard on {name} and scan the QR code.', {name: p.name}))}</p><div class="qr-container"><img src="${qrBlob}" alt="${esc(t('WireGuard configuration QR code for {name}', {name: p.name}))}" width="280" height="280"/></div><p class="qr-warning">${icon('lock')}${t("This QR code contains the device access key. Do not share it.")}</p><div class="modal-actions"><button class="btn secondary" id="qr-download">${icon('down')}${t("Download as file")}</button><button class="btn primary" id="qr-done">${t("Done")}</button></div>`, 'qr-modal');
  modal.querySelector('#qr-done')!.addEventListener('click', closeDialog); modal.querySelector('#qr-download')!.addEventListener('click', () => action(() => downloadConfig(id)));
}
function download(blob: Blob, filename: string): void { const url = URL.createObjectURL(blob); const a = document.createElement('a'); a.href = url; a.download = filename; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000); }
async function downloadConfig(id: string): Promise<void> { const response = await fetch(`/api/peers/${id}/config`, { credentials: 'same-origin' }); if (!response.ok) { const error = await response.json(); throw new Error(serverMessage(error.error)); } download(await response.blob(), `wireguard-${id}.conf`); toast(t("Configuration downloaded")); }
function confirmDialog(title: string, text: string, label: string, fn: () => Promise<void>): void { openDialog(title, `<p class="modal-subtitle">${text}</p><div class="modal-actions"><button class="btn secondary" id="cancel-confirm">${t("Cancel")}</button><button class="btn danger" id="confirm-action">${label}</button></div>`); modal.querySelector('#cancel-confirm')!.addEventListener('click', closeDialog); modal.querySelector<HTMLButtonElement>('#confirm-action')!.addEventListener('click', event => { const button = event.currentTarget as HTMLButtonElement; button.disabled = true; void action(fn).finally(() => { button.disabled = false; }); }); }
function importDialog(): void {
  openDialog(t("Import server configuration"), `<p class="modal-subtitle">${t("Select a server .conf file or paste its contents. This replaces the current client list.")}</p><form id="import-form"><label class="file-picker">${icon('upload')}<span>${t("Choose a .conf file")}</span><input id="import-file" type="file" accept=".conf,.txt"/></label><label class="textarea-label">${t("Server configuration")}<textarea name="config" rows="9" spellcheck="false" placeholder="[Interface]&#10;PrivateKey = …&#10;Address = 10.64.0.1/24&#10;ListenPort = 51820" required></textarea></label><label class="checkbox-row"><input type="checkbox" required/><span>${t("I understand that the current client list will be replaced.")}</span></label><div class="modal-note">${icon('shield')}${t("Hooks and commands are rejected. Server files do not contain client private keys.")}</div><div class="modal-actions"><button class="btn secondary" type="button" id="import-cancel">${t("Cancel")}</button><button class="btn primary" type="submit">${t("Import")}</button></div></form>`, 'wide-modal');
  modal.querySelector('#import-cancel')!.addEventListener('click', closeDialog);
  modal.querySelector<HTMLInputElement>('#import-file')!.addEventListener('change', event => { const file = (event.target as HTMLInputElement).files?.[0]; if (!file) return; void action(async () => { if (file.size > 256 * 1024) throw new Error(t("File must be at most 256 KB")); modal.querySelector<HTMLTextAreaElement>('textarea')!.value = await file.text(); }); });
  modal.querySelector<HTMLFormElement>('#import-form')!.addEventListener('submit', event => { event.preventDefault(); const form = event.currentTarget as HTMLFormElement; void submitForm(form, async () => { await api('/api/import', 'POST', { config: new FormData(form).get('config') }); closeDialog(); await refresh(true); toast(t("Configuration imported")); }); });
}

function renderAuth(): void {
  const setup = session.setupRequired;
  app.innerHTML = `<div class="auth-page"><aside class="auth-story"><div class="auth-wordmark"><img src="/favicon.svg" alt=""/>wireguard simple</div><div><h1>WireGuard Simple</h1><p>${t("WireGuard management, live visibility, and optional recording. One lightweight application.")}</p><div class="auth-features"><span>${icon('shield')}${t("Independent keys")}</span><span>${icon('pulse')}${t("Live monitoring")}</span><span>${icon('leaf')}${t("Bounded resource use")}</span></div></div><small>${t("WireGuard Simple · Independent VPN management")}</small></aside><section class="auth-form-wrap"><div class="auth-language">${languageControl()}</div><form id="auth-form"><span class="auth-lock">${icon('lock')}</span><div class="eyebrow">${setup ? t("FIRST-TIME SETUP") : t("ADMINISTRATOR ACCESS")}</div><h2>${setup ? t("Set up your console.") : t("Welcome back.")}</h2><p>${setup ? t("Create the first administrator account with the server's private setup token.") : t("Sign in to manage your network.")}</p>${setup ? `<label>${t("Setup token")}<input name="token" type="password" autocomplete="off" required/><small>${t("In the setup-token file in the server data directory.")}</small></label>` : ''}<label>${t("Username")}<input name="username" autocomplete="username" maxlength="40" required/></label><label>${t("Password")}<input name="password" type="password" autocomplete="${setup ? 'new-password' : 'current-password'}" ${setup ? 'minlength="12"' : ''} maxlength="256" required/>${setup ? `<small>${t("Use at least 12 characters.")}</small>` : ''}</label><button type="submit" class="btn primary full">${setup ? t("Create administrator account") : t("Sign in")}${icon('arrow')}</button><div class="auth-security">${icon('shield')}${t("Access is restricted to your administrator account.")}</div></form></section></div>`;
  bindLanguageControl();
  document.querySelector<HTMLFormElement>('#auth-form')!.addEventListener('submit', event => { event.preventDefault(); const form = event.currentTarget as HTMLFormElement; void submitForm(form, async () => { const f = new FormData(form); await api(setup ? '/api/setup' : '/api/login', 'POST', Object.fromEntries(f)); await start(); }); });
}
async function start(): Promise<void> {
  session = await api<Session>('/api/session');
  if (!session.authenticated) { renderAuth(); return; }
  const requested = location.hash.slice(1); if (['overview', 'peers', 'activity', 'storage', 'settings'].includes(requested)) view = requested as View;
  await refresh(true); renderMain(); updateChrome(); clearInterval(poll); poll = setInterval(() => { void refresh(); }, 3000);
}
document.addEventListener('visibilitychange', () => { if (!document.hidden && session?.authenticated) void refresh(); });
document.addEventListener('keydown', event => { if (event.key === '/' && !modal.open && !(document.activeElement instanceof HTMLInputElement || document.activeElement instanceof HTMLTextAreaElement)) { event.preventDefault(); if (view !== 'peers') navigate('peers'); document.querySelector<HTMLInputElement>('#peer-search')?.focus(); } });
window.addEventListener('hashchange', () => { const v = location.hash.slice(1); if (session?.authenticated && ['overview', 'peers', 'activity', 'storage', 'settings'].includes(v) && v !== view) { view = v as View; renderMain(); updateChrome(); } });
initLanguage();
const loading = document.querySelector('.boot p'); if (loading) loading.textContent = t('Loading network console…');
void start().catch(error => { app.innerHTML = `<div class="boot"><span class="boot-mark">W</span><h2>${t("Cannot connect to the console")}</h2><p>${esc(error.message)}</p><button class="btn secondary" id="retry">${t("Retry")}</button></div>`; document.querySelector('#retry')!.addEventListener('click', () => location.reload()); });
