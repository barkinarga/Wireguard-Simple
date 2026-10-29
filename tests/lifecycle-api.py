"""Private fixture for the hosted Linux installer test; never print credentials."""
import hashlib
import http.cookiejar
import json
import secrets
import ssl
import sqlite3
import subprocess
import sys
import urllib.request
from pathlib import Path


def main():
    action, work_dir = sys.argv[1:]
    work = Path(work_dir)
    state = Path('/var/lib/wgsimple')
    context = ssl.create_default_context(cafile=str(work / 'panel.crt'))
    client = urllib.request.build_opener(
        urllib.request.ProxyHandler({}),
        urllib.request.HTTPSHandler(context=context),
        urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()),
    )
    csrf = ''

    def request(path, data=None, method=None, raw=False):
        headers = {'Content-Type': 'application/json', 'X-CSRF-Token': csrf}
        req = urllib.request.Request('https://127.0.0.1:8787' + path,
            data=None if data is None else json.dumps(data).encode(),
            headers=headers, method=method)
        with client.open(req, timeout=10) as response:
            body = response.read()
            return body if raw else json.loads(body)

    credentials_file = work / 'credentials.json'
    if action == 'create':
        credentials = {'username': 'ci-admin', 'password': secrets.token_urlsafe(24)}
        credentials_file.write_text(json.dumps(credentials))
        csrf = request('/api/setup', dict(credentials,
            token=(state / 'setup-token').read_text().strip()))['csrf']
        snapshot = request('/api/snapshot')
        server = snapshot['server']
        server.update(endpoint='198.18.0.2', dns='')
        request('/api/server', server, 'PUT')
        request('/api/settings', dict(historyEnabled=False, auditEnabled=True,
            metricsEnabled=False, retentionDays=3, budgetMB=16, sampleSeconds=30), 'PUT')
        peer = request('/api/peers', {'name': 'installer-test-client'})
        (work / 'peer-id').write_text(peer['id'])
    else:
        credentials = json.loads(credentials_file.read_text())
        csrf = request('/api/login', credentials)['csrf']
        if request('/api/session')['setupRequired']:
            raise RuntimeError('Administrator was lost')

    peer_id = (work / 'peer-id').read_text()
    config = request('/api/peers/' + peer_id + '/config', raw=True)
    qr = request('/api/peers/' + peer_id + '/qr', raw=True)
    if not qr.startswith(b'\x89PNG\r\n\x1a\n'):
        raise RuntimeError('QR export failed')
    # Compare stable control records, including private keys and password hash,
    # without writing them to logs. Counters/audit events may legitimately change.
    db = sqlite3.connect('file:/var/lib/wgsimple/control.db?mode=ro', uri=True)
    records = {}
    for table in ('config', 'peers', 'admin'):
        rows = db.execute('SELECT * FROM ' + table).fetchall()
        records[table] = sorted(rows, key=repr)
    db.close()
    records['export'] = config.decode()
    records['tls'] = hashlib.sha256((state / 'panel-tls.pem').read_bytes()).hexdigest()
    baseline = work / 'baseline.json'
    if action == 'create':
        baseline.write_text(json.dumps(records, sort_keys=True))
        (work / 'client.conf').write_bytes(config)
        # wg consumes a stripped configuration; addresses/routes are added in
        # the isolated client namespace by the shell driver.
        stripped = subprocess.run(['wg-quick', 'strip', str(work / 'client.conf')],
            capture_output=True, check=True).stdout
        (work / 'client-wg.conf').write_bytes(stripped)
    elif json.loads(baseline.read_text()) != json.loads(json.dumps(records)):
        raise RuntimeError('Account, client, settings, keys, export or TLS identity changed')
    print('PASS HTTPS account, settings, client configuration and QR' if action == 'create'
        else 'PASS existing login, settings, client keys/configuration and TLS identity preserved')


if __name__ == '__main__':
    try:
        main()
    except Exception as exc:
        # HTTP response bodies and control data can contain secrets.
        print('Lifecycle API check failed: ' + type(exc).__name__
            + (' (HTTP ' + str(exc.code) + ')' if hasattr(exc, 'code') else ''), file=sys.stderr)
        sys.exit(1)
