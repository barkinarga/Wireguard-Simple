import test from 'node:test';
import assert from 'node:assert/strict';
import { build } from 'esbuild';
const bundle = await build({ entryPoints: ['ui/client-address.ts'], bundle: true, write: false, platform: 'node', format: 'esm' });
const { clientAddressPreview: preview, clientAddressIssue: issue, normalizeClientAddress: normalize } = await import('data:text/javascript;base64,' + Buffer.from(bundle.outputFiles[0].contents).toString('base64'));
const peers = (...addresses) => addresses.map(address => ({ address }));

test('suggests the first gap, reserving the server and every existing client', () => {
  assert.deepEqual(preview('10.64.0.1/24', []), { subnet: '10.64.0.0/24', next: '10.64.0.2/32' });
  assert.equal(preview('10.64.0.1/24', peers('10.64.0.2/32', '10.64.0.4/32')).next, '10.64.0.3/32');
  assert.equal(preview('10.8.4.6/29', peers('10.8.4.1/32, 10.8.4.2/32')).next, '10.8.4.3/32');
  assert.equal(preview('192.168.42.1/24', []).next, '192.168.42.2/32');
});
test('no proposal for exhausted pools or routed subnets; manual addresses remain checked', () => {
  assert.equal(preview('10.0.0.1/30', peers('10.0.0.2/32')).next, '');
  assert.equal(issue('10.0.0.1/30', peers('10.0.0.2/32'), ''), 'No available client address in the server subnet');
  assert.equal(preview('10.0.0.1/24', peers('10.0.0.128/25')).next, '');
  assert.ok(issue('10.0.0.1/24', peers('10.0.0.128/25'), '10.0.0.130/32'));
  assert.equal(issue('10.0.0.1/24', peers('10.0.0.128/25'), '10.0.0.2/32'), '');
  assert.equal(preview('10.0.0.0/31', []).next, '');
  assert.equal(preview('invalid', []).next, '');
});
test('validates manual IPs and rejects server, reserved, overlapping and foreign addresses', () => {
  const occupied = peers('10.64.0.2/32');
  for (const raw of ['10.64.0.0/32', '10.64.0.255/32', '10.64.0.1/32', '10.64.0.2/32', '10.65.0.2/32', '10.64.0.8/24', '10.64.0.256', '010.64.0.8', '::1/128', '1e2.64.0.8', 'bad']) assert.ok(issue('10.64.0.1/24', occupied, raw), raw);
  assert.equal(issue('10.64.0.1/24', occupied, '10.64.0.50'), '');
  assert.equal(normalize(' 10.64.0.50 '), '10.64.0.50/32');
  assert.equal(normalize('10.64.0.50/32'), '10.64.0.50/32');
  assert.equal(normalize(''), '');
});
test('an address suggested before another creation is subsequently rejected', () => {
  const proposed = preview('10.64.0.1/24', []).next;
  assert.equal(issue('10.64.0.1/24', peers(proposed), proposed), 'Client address is already in use');
  assert.equal(preview('10.64.0.1/24', peers(proposed)).next, '10.64.0.3/32');
});
