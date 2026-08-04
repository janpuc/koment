'use strict';

const fs = require('fs');
const path = require('path');

const bundledDirectory = 'bin';
const executableBits = 0o111;

function bundledBinary(extensionPath, platform) {
  const name = platform === 'win32' ? 'koment.exe' : 'koment';
  return path.join(extensionPath, bundledDirectory, name);
}

function resolveBinary({ configured, extensionPath, platform, exists = fs.existsSync }) {
  const explicit = (configured ?? '').trim();
  if (explicit) {
    return { command: explicit, source: 'configured' };
  }
  const bundled = bundledBinary(extensionPath, platform);
  if (exists(bundled)) {
    return { command: bundled, source: 'bundled' };
  }
  return { command: 'koment', source: 'path' };
}

function ensureExecutable(binary, { platform = process.platform, stat = fs.statSync, chmod = fs.chmodSync } = {}) {
  if (platform === 'win32') {
    return false;
  }
  const { mode } = stat(binary);
  if ((mode & executableBits) === executableBits) {
    return false;
  }
  chmod(binary, mode | executableBits);
  return true;
}

module.exports = { bundledBinary, ensureExecutable, resolveBinary };
