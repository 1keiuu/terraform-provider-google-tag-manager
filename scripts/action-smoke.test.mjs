import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { generateKeyPairSync, verify } from 'node:crypto';
import { createServer } from 'node:http';
import { mkdtemp, mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const actions = path.join(root, '.github/actions');

async function execute(command, args, env, cwd) {
  return await new Promise((resolve, reject) => {
    const child = spawn(command, args, { env, cwd, stdio: ['ignore', 'pipe', 'pipe'] });
    let stdout = '';
    let stderr = '';
    const timer = setTimeout(() => child.kill('SIGKILL'), 30_000);
    child.stdout.on('data', chunk => { stdout += chunk; });
    child.stderr.on('data', chunk => { stderr += chunk; });
    child.on('error', reject);
    child.on('close', code => {
      clearTimeout(timer);
      if (code === 0) resolve(stdout);
      else reject(new Error(`${path.basename(command)} exited ${code}: ${(stdout + stderr).slice(-3000)}`));
    });
  });
}

async function environment(directory) {
  const env = Object.fromEntries(['PATH', 'HOME', 'LANG', 'TMPDIR', 'SystemRoot']
    .filter(key => process.env[key]).map(key => [key, process.env[key]]));
  Object.assign(env, {
    GITHUB_ACTIONS: 'true', GITHUB_REPOSITORY: 'fixture/repository',
    GITHUB_REPOSITORY_OWNER: 'fixture', GITHUB_WORKSPACE: directory,
    RUNNER_TEMP: directory, RUNNER_OS: process.platform === 'darwin' ? 'macOS' : 'Linux',
  });
  for (const key of ['GITHUB_OUTPUT', 'GITHUB_STATE', 'GITHUB_ENV', 'GITHUB_PATH']) {
    env[key] = path.join(directory, key);
    await writeFile(env[key], '');
  }
  return env;
}

function savedState(content) {
  const result = {};
  const lines = content.split('\n');
  for (let i = 0; i < lines.length; i++) {
    const match = /^(\w+)<<(.+)$/.exec(lines[i]);
    if (!match) continue;
    const values = [];
    while (++i < lines.length && lines[i] !== match[2]) values.push(lines[i]);
    result[`STATE_${match[1]}`] = values.join('\n');
  }
  return result;
}

test('bundled App token Action restricts scope and revokes its token', { timeout: 60_000 }, async () => {
  const directory = await mkdtemp('/tmp/gtm-token-');
  const { privateKey, publicKey } = generateKeyPairSync('rsa', {
    modulusLength: 2048, privateKeyEncoding: { type: 'pkcs8', format: 'pem' },
    publicKeyEncoding: { type: 'spki', format: 'pem' },
  });
  const requests = [];
  const server = createServer(async (request, response) => {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const body = Buffer.concat(chunks).toString();
    requests.push({ method: request.method, url: request.url, body, authorization: request.headers.authorization });
    response.setHeader('Content-Type', 'application/json');
    if (request.url === '/repos/fixture/repository/installation') {
      response.end(JSON.stringify({ id: 123, app_slug: 'fixture-app' }));
    } else if (request.url === '/app/installations/123/access_tokens') {
      response.end(JSON.stringify({ token: 'fixture-installation-token', expires_at: new Date(Date.now() + 3_600_000).toISOString() }));
    } else if (request.url === '/installation/token' && request.method === 'DELETE') {
      response.statusCode = 204;
      response.end();
    } else {
      response.statusCode = 404;
      response.end('{}');
    }
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  try {
    const env = { ...await environment(directory),
      'INPUT_CLIENT-ID': '123', 'INPUT_PRIVATE-KEY': privateKey,
      'INPUT_GITHUB-API-URL': `http://127.0.0.1:${server.address().port}`,
      'INPUT_PERMISSION-CONTENTS': 'write', 'INPUT_SKIP-TOKEN-REVOKE': 'false',
    };
    await execute(process.execPath, [path.join(actions, 'create-github-app-token/dist/main.cjs')], env, directory);
    const creation = requests.find(request => request.method === 'POST');
    assert.deepEqual(JSON.parse(creation.body), { repositories: ['repository'], permissions: { contents: 'write' } });
    const jwt = creation.authorization.replace(/^bearer /i, '').split('.');
    assert.equal(verify('RSA-SHA256', Buffer.from(jwt.slice(0, 2).join('.')), publicKey, Buffer.from(jwt[2], 'base64url')), true);
    const state = savedState(await readFile(env.GITHUB_STATE, 'utf8'));
    assert.equal(state.STATE_token, 'fixture-installation-token');
    await execute(process.execPath, [path.join(actions, 'create-github-app-token/dist/post.cjs')], { ...env, ...state }, directory);
    const revocation = requests.find(request => request.method === 'DELETE');
    assert.equal(revocation.url, '/installation/token');
    assert.equal(revocation.authorization, 'token fixture-installation-token');
    assert.equal(requests.length, 3);
  } finally {
    await new Promise(resolve => server.close(resolve));
    await rm(directory, { recursive: true, force: true });
  }
});

test('bundled GPG Action imports, signs with, and removes an ephemeral key', { timeout: 90_000 }, async () => {
  const directory = await mkdtemp('/tmp/gtm-gpg-');
  const sourceHome = path.join(directory, 'source');
  const targetHome = path.join(directory, 'target');
  await mkdir(sourceHome, { mode: 0o700 });
  await mkdir(targetHome, { mode: 0o700 });
  const base = await environment(directory);
  const sourceEnv = { ...base, GNUPGHOME: sourceHome };
  const targetEnv = { ...base, GNUPGHOME: targetHome };
  try {
    await execute('gpg', ['--batch', '--pinentry-mode', 'loopback', '--passphrase', 'fixture-passphrase',
      '--quick-generate-key', 'CI Fixture <ci@example.invalid>', 'ed25519', 'sign', '1d'], sourceEnv, directory);
    const secret = await execute('gpg', ['--batch', '--pinentry-mode', 'loopback', '--passphrase', 'fixture-passphrase',
      '--armor', '--export-secret-keys', 'ci@example.invalid'], sourceEnv, directory);
    const env = { ...targetEnv, INPUT_GPG_PRIVATE_KEY: secret, INPUT_PASSPHRASE: 'fixture-passphrase',
      INPUT_GIT_CONFIG_GLOBAL: 'false', INPUT_GIT_USER_SIGNINGKEY: 'false',
      INPUT_GIT_COMMIT_GPGSIGN: 'false', INPUT_GIT_TAG_GPGSIGN: 'false',
    };
    const entry = path.join(actions, 'import-gpg/dist/index.js');
    await execute(process.execPath, [entry], env, directory);
    const state = savedState(await readFile(env.GITHUB_STATE, 'utf8'));
    assert.match(state.STATE_fingerprint, /^[A-F0-9]{40}$/);
    const message = path.join(directory, 'message');
    await writeFile(message, 'fixture signature verification\n');
    await execute('gpg', ['--batch', '--yes', '--local-user', state.STATE_fingerprint,
      '--output', `${message}.sig`, '--detach-sign', message], targetEnv, directory);
    await execute('gpg', ['--batch', '--verify', `${message}.sig`, message], targetEnv, directory);
    await execute(process.execPath, [entry], { ...env, ...state }, directory);
    const remaining = await execute('gpg', ['--batch', '--with-colons', '--list-secret-keys'], targetEnv, directory);
    assert.equal(remaining.includes(state.STATE_fingerprint), false);
  } finally {
    for (const env of [sourceEnv, targetEnv]) {
      await execute('gpgconf', ['--kill', 'gpg-agent'], env, directory).catch(() => {});
    }
    await rm(directory, { recursive: true, force: true });
  }
});
