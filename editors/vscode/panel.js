'use strict';

const statusTone = {
  ok: 'ok',
  moved: 'moved',
  ambiguous: 'failing',
  drifted: 'failing',
  orphaned: 'failing'
};

function escape(text) {
  return String(text ?? '').replace(
    /[&<>"']/g,
    (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character]
  );
}

function paragraphs(body) {
  return String(body ?? '')
    .split(/\n{2,}/)
    .map((block) => block.trim())
    .filter(Boolean)
    .map((block) => `<p>${escape(block).replace(/\n/g, '<br>')}</p>`)
    .join('');
}

function entry(item, index) {
  const tone = statusTone[item.status] ?? 'failing';
  const warning = item.warning ? `<p class="warning">${escape(item.warning)}</p>` : '';
  return `<li>
  <button type="button" data-reveal="${index}">
    <span class="title">${escape(item.title)}</span>
    <span class="meta"><span class="kind">${escape(item.kind)}</span><span class="status ${tone}">${escape(item.status)}</span><span class="line">L${Number(item.line) || 0}</span></span>
    <span class="body">${paragraphs(item.body)}${warning}</span>
  </button>
</li>`;
}

// The panel exists because a body is a paragraph and every inline surface VS
// Code offers is one line (ADR 0114). Rendering prose is the whole job, so the
// markup is prose, not rows.
function panelHTML({ items, file, nonce, styleNonce }) {
  const list = items.length
    ? `<ul>${items.map(entry).join('')}</ul>`
    : `<p class="empty">No annotations in this file.</p>`;
  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'nonce-${styleNonce}'; script-src 'nonce-${nonce}';">
<style nonce="${styleNonce}">
  :root { color-scheme: light dark; }
  body {
    margin: 0; padding: 0.4rem 0.2rem;
    font-family: var(--vscode-font-family); font-size: var(--vscode-font-size);
    color: var(--vscode-foreground); background: transparent;
  }
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 0.15rem; }
  button {
    display: block; width: 100%; text-align: left; cursor: pointer;
    background: none; border: none; border-left: 2px solid transparent;
    padding: 0.45rem 0.6rem; color: inherit; font: inherit;
  }
  button:hover { background: var(--vscode-list-hoverBackground); }
  button:focus-visible { outline: 1px solid var(--vscode-focusBorder); outline-offset: -1px; }
  .title { display: block; font-weight: 600; margin-bottom: 0.15rem; }
  .meta { display: flex; gap: 0.5rem; align-items: baseline; margin-bottom: 0.3rem; }
  .kind { color: var(--vscode-descriptionForeground); }
  .status { font-size: 0.85em; text-transform: uppercase; letter-spacing: 0.04em; }
  .status.ok { color: var(--vscode-descriptionForeground); }
  .status.moved { color: var(--vscode-editorWarning-foreground); }
  .status.failing { color: var(--vscode-editorError-foreground); }
  .line { margin-left: auto; color: var(--vscode-descriptionForeground); font-variant-numeric: tabular-nums; }
  .body p { margin: 0 0 0.4rem; line-height: 1.45; white-space: normal; }
  .body p:last-child { margin-bottom: 0; }
  .warning { color: var(--vscode-editorError-foreground); }
  .empty { color: var(--vscode-descriptionForeground); padding: 0.6rem; }
  .file { color: var(--vscode-descriptionForeground); padding: 0 0.6rem 0.4rem; }
</style>
</head>
<body>
${file ? `<p class="file">${escape(file)}</p>` : ''}
${list}
<script nonce="${nonce}">
  const vscode = acquireVsCodeApi();
  document.addEventListener('click', (event) => {
    const target = event.target.closest('[data-reveal]');
    if (target) {
      vscode.postMessage({ reveal: Number(target.dataset.reveal) });
    }
  });
</script>
</body>
</html>`;
}

module.exports = { escape, panelHTML, paragraphs };
