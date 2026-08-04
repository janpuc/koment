'use strict';

const vscode = require('vscode');
const { ensureExecutable, resolveBinary } = require('./binary');
const { ProtocolClient } = require('./protocol');

let client;
let diagnostics;
let annotationDecoration;
let output;
const changedDocuments = new Set();
const savedDocuments = new Set();
const knownComments = new Map();
const pendingCommentIntent = new Map();
const refreshTimers = new Map();

async function activate(context) {
  output = vscode.window.createOutputChannel('koment');
  diagnostics = vscode.languages.createDiagnosticCollection('koment');
  annotationDecoration = vscode.window.createTextEditorDecorationType({
    after: { color: new vscode.ThemeColor('editorCodeLens.foreground'), margin: '0 0 0 1rem' },
    overviewRulerColor: new vscode.ThemeColor('editorOverviewRuler.infoForeground'),
    overviewRulerLane: vscode.OverviewRulerLane.Right
  });
  context.subscriptions.push(output, diagnostics, annotationDecoration);

  const configuration = vscode.workspace.getConfiguration('koment');
  const server = resolveBinary({
    configured: configuration.get('binaryPath', ''),
    extensionPath: context.extensionPath,
    platform: process.platform
  });
  output.appendLine(`koment: starting ${server.command} (${server.source})`);
  if (server.source === 'bundled') {
    ensureExecutable(server.command);
  }
  const workspaceFolders = vscode.workspace.workspaceFolders ?? [];
  const cwd = workspaceFolders[0]?.uri.fsPath;
  client = new ProtocolClient(server.command, ['lsp'], {
    cwd,
    env: process.env,
    onStderr: (text) => output.append(text),
    onError: (error) => showServerError(error)
  });
  try {
    await client.start();
    await client.request('initialize', {
      processId: process.pid,
      clientInfo: { name: 'koment-vscode', version: context.extension.packageJSON.version },
      rootUri: workspaceFolders[0]?.uri.toString() ?? null,
      workspaceFolders: workspaceFolders.map((folder) => ({ uri: folder.uri.toString(), name: folder.name })),
      capabilities: {}
    });
    client.notify('initialized', {});
  } catch (error) {
    showStartupError(error);
    return;
  }

  context.subscriptions.push(client.onNotification('textDocument/publishDiagnostics', publishDiagnostics));
  registerCommands(context);
  registerLanguageFeatures(context);
  registerDocumentEvents(context);
  for (const document of vscode.workspace.textDocuments.filter(isFileDocument)) {
    openDocument(document);
  }
  for (const editor of vscode.window.visibleTextEditors) {
    refreshAnnotations(editor.document);
  }
}

function registerCommands(context) {
  context.subscriptions.push(
    vscode.commands.registerCommand('koment.add', addAnnotation),
    vscode.commands.registerCommand('koment.reanchor', reanchorAnnotation),
    vscode.commands.registerCommand('koment.convertComment', convertComment),
    vscode.commands.registerCommand('koment.acknowledgeComment', acknowledgeComment),
    vscode.commands.registerCommand('koment.showAnnotation', showAnnotation)
  );
}

function registerLanguageFeatures(context) {
  const selector = { scheme: 'file' };
  context.subscriptions.push(
    vscode.languages.registerHoverProvider(selector, { provideHover }),
    vscode.languages.registerCodeLensProvider(selector, { provideCodeLenses }),
    vscode.languages.registerCodeActionsProvider(selector, { provideCodeActions }, {
      providedCodeActionKinds: [vscode.CodeActionKind.QuickFix]
    })
  );
}

function registerDocumentEvents(context) {
  context.subscriptions.push(
    vscode.workspace.onDidOpenTextDocument(openDocument),
    vscode.workspace.onDidChangeTextDocument(({ document }) => {
      if (!isFileDocument(document)) {
        return;
      }
      changedDocuments.add(document.uri.toString());
      client.notify('textDocument/didChange', {
        textDocument: { uri: document.uri.toString(), version: document.version },
        contentChanges: [{ text: document.getText() }]
      });
      scheduleAnnotationRefresh(document);
    }),
    vscode.workspace.onDidSaveTextDocument((document) => {
      if (!isFileDocument(document)) {
        return;
      }
      savedDocuments.add(document.uri.toString());
      client.notify('textDocument/didSave', {
        textDocument: { uri: document.uri.toString() }, text: document.getText()
      });
    }),
    vscode.workspace.onDidCloseTextDocument((document) => {
      if (!isFileDocument(document)) {
        return;
      }
      const uri = document.uri.toString();
      client.notify('textDocument/didClose', { textDocument: { uri } });
      knownComments.delete(uri);
      pendingCommentIntent.delete(uri);
    }),
    vscode.window.onDidChangeVisibleTextEditors((editors) => {
      for (const editor of editors) {
        refreshAnnotations(editor.document);
      }
    })
  );
}

function openDocument(document) {
  if (!isFileDocument(document)) {
    return;
  }
  client.notify('textDocument/didOpen', {
    textDocument: {
      uri: document.uri.toString(), languageId: document.languageId,
      version: document.version, text: document.getText()
    }
  });
}

function isFileDocument(document) {
  return document.uri.scheme === 'file';
}

function publishDiagnostics(params) {
  const uri = vscode.Uri.parse(params.uri);
  const converted = (params.diagnostics ?? []).map((problem) => {
    const diagnostic = new vscode.Diagnostic(
      toRange(problem.range), problem.message, toDiagnosticSeverity(problem.severity)
    );
    diagnostic.code = problem.code;
    diagnostic.source = problem.source;
    diagnostic.komentRaw = problem;
    return diagnostic;
  });
  diagnostics.set(uri, converted);
  trackCommentIntent(params.uri, params.diagnostics);
  refreshAnnotationsForURI(uri);
}

function trackCommentIntent(uri, problems) {
  const current = new Map();
  for (const problem of problems.filter((value) => value.code === 'koment.comment')) {
    const key = `${problem.range.start.line}:${problem.data?.comment ?? ''}`;
    current.set(key, problem.data);
    if (changedDocuments.has(uri) && !knownComments.get(uri)?.has(key) && problem.data?.autoPrompt) {
      const pending = pendingCommentIntent.get(uri) ?? new Map();
      pending.set(key, problem.data);
      pendingCommentIntent.set(uri, pending);
    }
  }
  knownComments.set(uri, new Set(current.keys()));
  if (!savedDocuments.delete(uri)) {
    return;
  }
  changedDocuments.delete(uri);
  const pending = pendingCommentIntent.get(uri);
  pendingCommentIntent.delete(uri);
  if (vscode.workspace.getConfiguration('koment').get('promptOnCommentIntent', true) && pending?.size) {
    promptForCommentIntent(uri, pending.values().next().value);
  }
}

async function promptForCommentIntent(uri, data) {
  const choice = await vscode.window.showInformationMessage(
    'This explanatory comment can become a koment annotation without staying in the source.',
    'Convert to koment', 'Keep inline', 'Not now'
  );
  if (choice === 'Convert to koment') {
    await convertComment({ uri, comment: data.comment });
  } else if (choice === 'Keep inline') {
    await acknowledgeComment({ uri, comment: data.comment });
  }
}

async function provideHover(document, position) {
  const result = await client.request('textDocument/hover', {
    textDocument: { uri: document.uri.toString() }, position: fromPosition(position)
  });
  if (!result) {
    return undefined;
  }
  const markdown = new vscode.MarkdownString(result.contents.value);
  markdown.isTrusted = false;
  return new vscode.Hover(markdown, toRange(result.range));
}

async function provideCodeLenses(document) {
  const lenses = await client.request('textDocument/codeLens', {
    textDocument: { uri: document.uri.toString() }
  });
  return (lenses ?? []).map((lens) => new vscode.CodeLens(toRange(lens.range), lens.command));
}

async function provideCodeActions(document, range, context) {
  const response = await client.request('textDocument/codeAction', {
    textDocument: { uri: document.uri.toString() },
    range: fromRange(range),
    context: { diagnostics: context.diagnostics.map((problem) => problem.komentRaw).filter(Boolean) }
  });
  return (response ?? []).map((raw) => {
    const action = new vscode.CodeAction(raw.title, vscode.CodeActionKind.QuickFix);
    action.command = raw.command;
    action.isPreferred = raw.title === 'Convert to koment';
    return action;
  });
}

async function refreshAnnotations(document) {
  if (!isFileDocument(document)) {
    return;
  }
  let items;
  try {
    items = await client.request('koment/annotations', {
      textDocument: { uri: document.uri.toString() }
    });
  } catch (error) {
    output.appendLine(error.message);
    return;
  }
  const maximum = vscode.workspace.getConfiguration('koment').get('inlineMaxLength', 100);
  const decorations = (items ?? []).map((item) => {
    const body = oneLine(item.body, maximum);
    const status = item.status === 'ok' ? '💬' : `💬 ${item.status}`;
    const hoverMessage = new vscode.MarkdownString(`**${item.kind}** · \`${item.status}\`\n\n${item.body}`);
    hoverMessage.isTrusted = false;
    return {
      range: toRange(item.range), hoverMessage,
      renderOptions: { after: { contentText: `${status} ${body}` } }
    };
  });
  for (const editor of vscode.window.visibleTextEditors.filter((value) => value.document.uri.toString() === document.uri.toString())) {
    editor.setDecorations(annotationDecoration, decorations);
  }
}

function scheduleAnnotationRefresh(document) {
  const key = document.uri.toString();
  clearTimeout(refreshTimers.get(key));
  refreshTimers.set(key, setTimeout(() => {
    refreshTimers.delete(key);
    refreshAnnotations(document);
  }, 150));
}

function refreshAnnotationsForURI(uri) {
  const document = vscode.workspace.textDocuments.find((value) => value.uri.toString() === uri.toString());
  if (document) {
    refreshAnnotations(document);
  }
}

async function addAnnotation() {
  const editor = vscode.window.activeTextEditor;
  if (!editor || !isFileDocument(editor.document)) {
    return;
  }
  const body = await vscode.window.showInputBox({
    title: 'Add koment annotation', prompt: 'Rationale to keep outside the source',
    validateInput: (value) => value.trim() ? undefined : 'The annotation body is required.'
  });
  if (body === undefined) {
    return;
  }
  const kind = await vscode.window.showQuickPick(['why', 'gotcha', 'invariant', 'anti-pattern'], {
    title: 'Annotation kind', placeHolder: 'why'
  });
  if (!kind) {
    return;
  }
  const selected = editor.document.getText(editor.selection).trim();
  const currentLine = editor.document.lineAt(editor.selection.active.line).text.trim();
  const excerpt = await vscode.window.showInputBox({
    title: 'Anchor excerpt',
    prompt: 'Use unique source text, or leave empty to annotate the whole file.',
    value: selected || currentLine
  });
  if (excerpt === undefined) {
    return;
  }
  await executeMutation(commandArguments(editor.document, { body, kind, excerpt }), 'koment.add');
}

async function reanchorAnnotation() {
  const editor = vscode.window.activeTextEditor;
  if (!editor || !isFileDocument(editor.document)) {
    return;
  }
  const id = await vscode.window.showInputBox({ title: 'Reanchor koment annotation', prompt: 'Annotation ULID' });
  if (!id) {
    return;
  }
  const selected = editor.document.getText(editor.selection).trim();
  const excerpt = await vscode.window.showInputBox({
    title: 'New anchor excerpt', prompt: 'Use unique source text, or leave empty to keep the current excerpt.', value: selected
  });
  if (excerpt === undefined) {
    return;
  }
  await executeMutation(commandArguments(editor.document, { id, excerpt }), 'koment.reanchor');
}

async function convertComment(argumentsValue) {
  const document = await documentForArguments(argumentsValue);
  if (!document) {
    return;
  }
  await executeMutation({ ...argumentsValue, uri: document.uri.toString(), kind: argumentsValue?.kind ?? 'why' }, 'koment.convertComment');
}

async function acknowledgeComment(argumentsValue) {
  const document = await documentForArguments(argumentsValue);
  if (!document) {
    return;
  }
  const choice = await vscode.window.showWarningMessage(
    'Keeping this comment breaks the normal koment procedure. Confirm that renaming, extraction, a named type or constant, and restructuring cannot express it.',
    { modal: true }, 'Acknowledge and keep inline'
  );
  if (choice !== 'Acknowledge and keep inline') {
    return;
  }
  const body = await vscode.window.showInputBox({
    title: 'Inline-comment exception rationale',
    prompt: 'Why must this exact comment remain in the source?',
    validateInput: (value) => value.trim() ? undefined : 'The exception rationale is required.'
  });
  if (body === undefined) {
    return;
  }
  await executeMutation({
    ...argumentsValue, uri: document.uri.toString(), body, acknowledgeInlineComment: true
  }, 'koment.acknowledgeComment');
}

async function documentForArguments(argumentsValue) {
  const uri = argumentsValue?.uri ? vscode.Uri.parse(argumentsValue.uri) : vscode.window.activeTextEditor?.document.uri;
  if (!uri) {
    return undefined;
  }
  return vscode.workspace.openTextDocument(uri);
}

async function executeMutation(argumentsValue, command) {
  const document = await documentForArguments(argumentsValue);
  if (!document) {
    return;
  }
  if (document.isDirty && !await document.save()) {
    vscode.window.showErrorMessage('koment needs the document saved before changing annotations.');
    return;
  }
  try {
    const result = await client.request('workspace/executeCommand', { command, arguments: [argumentsValue] });
    if (result.fileChanged) {
      await synchronizeDocument(document);
    }
    await refreshAnnotations(document);
    const warning = result.warnings?.length ? ` ${result.warnings.join(' ')}` : '';
    vscode.window.showInformationMessage(`koment wrote ${result.id}.${warning}`);
  } catch (error) {
    vscode.window.showErrorMessage(`koment: ${error.message}`);
  }
}

async function synchronizeDocument(document) {
  const disk = Buffer.from(await vscode.workspace.fs.readFile(document.uri)).toString('utf8');
  if (disk === document.getText()) {
    return;
  }
  const edit = new vscode.WorkspaceEdit();
  edit.replace(document.uri, new vscode.Range(document.positionAt(0), document.positionAt(document.getText().length)), disk);
  if (!await vscode.workspace.applyEdit(edit) || !await document.save()) {
    throw new Error('annotation exists, but VS Code could not refresh the converted source');
  }
}

function commandArguments(document, values) {
  return { uri: document.uri.toString(), ...values };
}

function showAnnotation(item) {
  const warning = item.warning ? `\n\n${item.warning}` : '';
  vscode.window.showInformationMessage(`${item.kind} · ${item.status}\n\n${item.body}${warning}`);
}

function showStartupError(error) {
  output?.appendLine(error.stack ?? error.message);
  vscode.window.showErrorMessage(
    `koment could not start: ${error.message}. Install a released koment binary or set koment.binaryPath.`
  );
}

function showServerError(error) {
  output?.appendLine(error.stack ?? error.message);
  vscode.window.showErrorMessage(`koment stopped responding: ${error.message}. See the koment output channel.`);
}

function toDiagnosticSeverity(value) {
  return ({ 1: vscode.DiagnosticSeverity.Error, 2: vscode.DiagnosticSeverity.Warning,
    3: vscode.DiagnosticSeverity.Information, 4: vscode.DiagnosticSeverity.Hint })[value] ?? vscode.DiagnosticSeverity.Warning;
}

function toRange(value) {
  return new vscode.Range(toPosition(value.start), toPosition(value.end));
}

function toPosition(value) {
  return new vscode.Position(value.line, value.character);
}

function fromRange(value) {
  return { start: fromPosition(value.start), end: fromPosition(value.end) };
}

function fromPosition(value) {
  return { line: value.line, character: value.character };
}

function oneLine(value, maximum) {
  const flattened = value.replace(/\s+/g, ' ').trim();
  return flattened.length <= maximum ? flattened : `${flattened.slice(0, maximum - 1)}…`;
}

async function deactivate() {
  for (const timer of refreshTimers.values()) {
    clearTimeout(timer);
  }
  await client?.stop();
}

module.exports = { activate, deactivate };
