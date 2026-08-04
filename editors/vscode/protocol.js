'use strict';

const childProcess = require('node:child_process');

const headerLimit = 8192;
const messageLimit = 8 * 1024 * 1024;

class MessageReader {
  constructor(onMessage, onError) {
    this.buffer = Buffer.alloc(0);
    this.onMessage = onMessage;
    this.onError = onError;
  }

  append(chunk) {
    this.buffer = Buffer.concat([this.buffer, chunk]);
    try {
      this.drain();
    } catch (error) {
      this.buffer = Buffer.alloc(0);
      this.onError(error);
    }
  }

  drain() {
    while (this.buffer.length > 0) {
      const boundary = this.buffer.indexOf('\r\n\r\n');
      if (boundary < 0) {
        if (this.buffer.length > headerLimit) {
          throw new Error('koment LSP header exceeds 8192 bytes');
        }
        return;
      }
      const headers = this.buffer.subarray(0, boundary).toString('ascii').split('\r\n');
      const lengthHeader = headers.find((value) => value.toLowerCase().startsWith('content-length:'));
      if (!lengthHeader) {
        throw new Error('koment LSP response has no Content-Length');
      }
      const length = Number(lengthHeader.slice(lengthHeader.indexOf(':') + 1).trim());
      if (!Number.isSafeInteger(length) || length < 0 || length > messageLimit) {
        throw new Error(`invalid koment LSP Content-Length ${length}`);
      }
      const bodyStart = boundary + 4;
      if (this.buffer.length < bodyStart + length) {
        return;
      }
      const body = this.buffer.subarray(bodyStart, bodyStart + length);
      this.buffer = this.buffer.subarray(bodyStart + length);
      this.onMessage(JSON.parse(body.toString('utf8')));
    }
  }
}

function frame(message) {
  const body = Buffer.from(JSON.stringify(message), 'utf8');
  return Buffer.concat([Buffer.from(`Content-Length: ${body.length}\r\n\r\n`, 'ascii'), body]);
}

class ProtocolClient {
  constructor(command, args, options = {}) {
    this.command = command;
    this.args = args;
    this.options = options;
    this.nextID = 1;
    this.pending = new Map();
    this.handlers = new Map();
  }

  start() {
    return new Promise((resolve, reject) => {
      this.process = childProcess.spawn(this.command, this.args, {
        cwd: this.options.cwd,
        env: this.options.env,
        stdio: ['pipe', 'pipe', 'pipe'],
        windowsHide: true
      });
      const reader = new MessageReader(
        (message) => this.receive(message),
        (error) => this.fail(error)
      );
      this.process.stdout.on('data', (chunk) => reader.append(chunk));
      this.process.stderr.on('data', (chunk) => this.options.onStderr?.(chunk.toString('utf8')));
      this.process.once('spawn', resolve);
      this.process.once('error', reject);
      this.process.once('exit', (code, signal) => {
        if (code !== 0 && !this.stopping) {
          this.fail(new Error(`koment lsp exited with ${signal ?? `code ${code}`}`));
        }
      });
    });
  }

  request(method, params, timeoutMilliseconds = 15000) {
    const id = this.nextID++;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`${method} timed out`));
      }, timeoutMilliseconds);
      this.pending.set(id, { resolve, reject, timer });
      this.write({ jsonrpc: '2.0', id, method, params });
    });
  }

  notify(method, params) {
    this.write({ jsonrpc: '2.0', method, params });
  }

  onNotification(method, handler) {
    const handlers = this.handlers.get(method) ?? new Set();
    handlers.add(handler);
    this.handlers.set(method, handlers);
    return { dispose: () => handlers.delete(handler) };
  }

  async stop() {
    if (!this.process || this.stopping) {
      return;
    }
    this.stopping = true;
    try {
      await this.request('shutdown', null, 3000);
      this.notify('exit', null);
    } catch {
      this.process.kill();
    }
  }

  write(message) {
    if (!this.process?.stdin.writable) {
      throw new Error('koment lsp is not running');
    }
    this.process.stdin.write(frame(message));
  }

  receive(message) {
    if (Object.hasOwn(message, 'id')) {
      const pending = this.pending.get(message.id);
      if (!pending) {
        return;
      }
      clearTimeout(pending.timer);
      this.pending.delete(message.id);
      if (message.error) {
        pending.reject(new Error(message.error.message));
      } else {
        pending.resolve(message.result);
      }
      return;
    }
    for (const handler of this.handlers.get(message.method) ?? []) {
      handler(message.params);
    }
  }

  fail(error) {
    for (const pending of this.pending.values()) {
      clearTimeout(pending.timer);
      pending.reject(error);
    }
    this.pending.clear();
    this.options.onError?.(error);
  }
}

module.exports = { MessageReader, ProtocolClient, frame };
