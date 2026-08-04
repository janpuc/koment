'use strict';

const assert = require('node:assert');
const path = require('node:path');
const { test } = require('node:test');
const { bundledBinary, ensureExecutable, resolveBinary } = require('./binary');

const never = () => false;
const always = () => true;

test('an explicit setting wins over the bundled binary', () => {
  const resolved = resolveBinary({
    configured: '/opt/koment/koment',
    extensionPath: '/ext',
    platform: 'linux',
    exists: always
  });
  assert.deepStrictEqual(resolved, { command: '/opt/koment/koment', source: 'configured' });
});

test('a blank setting is not a path', () => {
  for (const configured of ['', '   ', undefined, null]) {
    const resolved = resolveBinary({ configured, extensionPath: '/ext', platform: 'linux', exists: always });
    assert.strictEqual(resolved.source, 'bundled', `${JSON.stringify(configured)} should not be taken as a path`);
  }
});

test('the bundled binary is used when the extension carries one', () => {
  const resolved = resolveBinary({ extensionPath: '/ext', platform: 'linux', exists: always });
  assert.deepStrictEqual(resolved, { command: path.join('/ext', 'bin', 'koment'), source: 'bundled' });
});

test('the universal package falls back to PATH', () => {
  const resolved = resolveBinary({ extensionPath: '/ext', platform: 'linux', exists: never });
  assert.deepStrictEqual(resolved, { command: 'koment', source: 'path' });
});

test('windows carries the executable suffix', () => {
  assert.strictEqual(bundledBinary('/ext', 'win32'), path.join('/ext', 'bin', 'koment.exe'));
  assert.strictEqual(bundledBinary('/ext', 'darwin'), path.join('/ext', 'bin', 'koment'));
});

// A VSIX is a zip, and the extension host is not required to restore the
// executable bit when it expands one. A bundled server that cannot be executed
// looks exactly like a missing server, so the bit is asserted before spawning.
test('a bundled binary without the executable bit is repaired', () => {
  const chmodded = [];
  const repaired = ensureExecutable('/ext/bin/koment', {
    platform: 'linux',
    stat: () => ({ mode: 0o644 }),
    chmod: (file, mode) => chmodded.push([file, mode])
  });
  assert.strictEqual(repaired, true);
  assert.deepStrictEqual(chmodded, [['/ext/bin/koment', 0o755]]);
});

test('an already executable binary is left alone', () => {
  const repaired = ensureExecutable('/ext/bin/koment', {
    platform: 'linux',
    stat: () => ({ mode: 0o755 }),
    chmod: () => assert.fail('should not chmod an executable file')
  });
  assert.strictEqual(repaired, false);
});

test('windows has no executable bit to repair', () => {
  const repaired = ensureExecutable('C:\\ext\\bin\\koment.exe', {
    platform: 'win32',
    stat: () => assert.fail('should not stat on windows'),
    chmod: () => assert.fail('should not chmod on windows')
  });
  assert.strictEqual(repaired, false);
});

test('a chmod failure is not swallowed', () => {
  assert.throws(
    () =>
      ensureExecutable('/ext/bin/koment', {
        platform: 'linux',
        stat: () => ({ mode: 0o644 }),
        chmod: () => {
          throw new Error('read-only file system');
        }
      }),
    /read-only file system/
  );
});
