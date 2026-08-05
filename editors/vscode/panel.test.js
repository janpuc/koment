'use strict';

const assert = require('node:assert');
const { test } = require('node:test');
const { escape, panelHTML, paragraphs } = require('./panel');

const render = (items) => panelHTML({ items, file: 'internal/store/ulid.go', nonce: 'n1', styleNonce: 's1' });

// The title is what the editor shows beside the code; the panel is where the
// body it stands for is actually read, so both have to be present.
test('an entry carries its title above its body', () => {
  const html = render([
    { kind: 'why', status: 'ok', line: 4, title: 'Retry once before giving up', body: 'The upstream closes idle connections.' }
  ]);
  assert.ok(html.includes('Retry once before giving up'));
  assert.ok(html.includes('The upstream closes idle connections.'));
  assert.ok(html.indexOf('Retry once') < html.indexOf('The upstream'), 'the title should precede the body');
});

test('a title cannot inject markup either', () => {
  const html = render([{ kind: 'why', status: 'ok', line: 1, title: '<img src=x onerror=y>', body: 'fine' }]);
  assert.ok(!html.includes('<img src=x'));
  assert.ok(html.includes('&lt;img src=x'));
});

// An annotation body is prose from the repository, and the panel puts it in a
// document. Anything that reaches the markup unescaped executes there.
test('a body cannot inject markup or script', () => {
  const html = render([
    { kind: 'why', status: 'ok', line: 3, body: '<script>alert(1)</script> & "quoted" <img src=x onerror=y>' }
  ]);
  assert.ok(!html.includes('<script>alert(1)</script>'), 'script tag survived escaping');
  assert.ok(!html.includes('<img src=x'), 'img tag survived escaping');
  assert.ok(html.includes('&lt;script&gt;alert(1)&lt;/script&gt;'));
  assert.ok(html.includes('&amp;'));
});

test('a kind or status cannot inject either', () => {
  const html = render([{ kind: '<b>why</b>', status: '"ok', line: 1, body: 'fine' }]);
  assert.ok(!html.includes('<b>why</b>'));
  assert.ok(html.includes('&lt;b&gt;why&lt;/b&gt;'));
});

// The panel exists to remove truncation, so a long body has to arrive whole.
test('the whole body is rendered, however long', () => {
  const body = 'word '.repeat(400).trim();
  const html = render([{ kind: 'invariant', status: 'ok', line: 9, body }]);
  assert.ok(html.includes(body), 'the body was shortened somewhere');
});

test('blank lines become separate paragraphs and single newlines stay inside one', () => {
  const html = paragraphs('first line\nstill first\n\nsecond block');
  assert.strictEqual(html, '<p>first line<br>still first</p><p>second block</p>');
});

test('an empty body renders nothing rather than an empty paragraph', () => {
  assert.strictEqual(paragraphs(''), '');
  assert.strictEqual(paragraphs(null), '');
  assert.strictEqual(paragraphs('   \n\n  '), '');
});

test('every entry can be revealed and carries its line', () => {
  const html = render([
    { kind: 'why', status: 'ok', line: 12, body: 'one' },
    { kind: 'gotcha', status: 'drifted', line: 40, body: 'two', warning: 'the excerpt is gone' }
  ]);
  assert.ok(html.includes('data-reveal="0"'));
  assert.ok(html.includes('data-reveal="1"'));
  assert.ok(html.includes('L12') && html.includes('L40'));
  assert.ok(html.includes('the excerpt is gone'));
});

test('a failing status is toned apart from a healthy one', () => {
  const html = render([
    { kind: 'why', status: 'ok', line: 1, body: 'a' },
    { kind: 'why', status: 'moved', line: 2, body: 'b' },
    { kind: 'why', status: 'orphaned', line: 3, body: 'c' }
  ]);
  assert.ok(html.includes('status ok'));
  assert.ok(html.includes('status moved'));
  assert.ok(html.includes('status failing'));
});

// A webview without a restrictive policy is a browser tab inside the editor.
test('the document restricts itself to its own nonces', () => {
  const html = render([{ kind: 'why', status: 'ok', line: 1, body: 'a' }]);
  assert.ok(html.includes("default-src 'none'"));
  assert.ok(html.includes("script-src 'nonce-n1'"));
  assert.ok(html.includes("style-src 'nonce-s1'"));
  assert.ok(!/<script(?![^>]*nonce=)/.test(html), 'a script tag carries no nonce');
});

test('an empty file says so instead of rendering an empty list', () => {
  const html = render([]);
  assert.ok(html.includes('No annotations in this file.'));
  assert.ok(!html.includes('<ul>'));
});

test('escape leaves ordinary prose alone', () => {
  assert.strictEqual(escape('plain prose, with punctuation.'), 'plain prose, with punctuation.');
});
