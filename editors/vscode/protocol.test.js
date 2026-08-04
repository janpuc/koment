'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');
const { MessageReader, frame } = require('./protocol');

test('reads fragmented and adjacent LSP frames', () => {
  const messages = [];
  const errors = [];
  const reader = new MessageReader((message) => messages.push(message), (error) => errors.push(error));
  const input = Buffer.concat([
    frame({ jsonrpc: '2.0', id: 1, result: { ready: true } }),
    frame({ jsonrpc: '2.0', method: 'textDocument/publishDiagnostics', params: { diagnostics: [] } })
  ]);
  reader.append(input.subarray(0, 11));
  reader.append(input.subarray(11, 47));
  reader.append(input.subarray(47));
  assert.deepEqual(errors, []);
  assert.equal(messages.length, 2);
  assert.equal(messages[0].result.ready, true);
  assert.equal(messages[1].method, 'textDocument/publishDiagnostics');
});

test('rejects an unbounded frame', () => {
  const errors = [];
  const reader = new MessageReader(() => assert.fail('message must not be emitted'), (error) => errors.push(error));
  reader.append(Buffer.from('Content-Length: 9000000\r\n\r\n'));
  assert.equal(errors.length, 1);
  assert.match(errors[0].message, /invalid koment LSP Content-Length/);
});
