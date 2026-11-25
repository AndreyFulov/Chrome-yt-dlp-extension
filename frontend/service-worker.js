const HOST_NAME = 'com.ytdlp.downloader';
const PROTOCOL_VERSION = 1;
const commands = new Set(['hello', 'inspect', 'start', 'status', 'cancel', 'chooseCookies', 'chooseDirectory', 'resetDirectory', 'restore']);
const activeStates = new Set(['downloading', 'processing']);
const pending = new Map();
let nativePort;
let diagnostics;
let diagnosticsPending;
let jobWrite = Promise.resolve();
let watchTimer;
let watching = false;

function connect() {
  if (nativePort) return nativePort;
  const port = chrome.runtime.connectNative(HOST_NAME);
  nativePort = port;
  port.onMessage.addListener((reply) => {
    const callback = pending.get(reply?.id);
    if (callback) callback(reply.protocolVersion === PROTOCOL_VERSION ? reply : { error: 'protocol_mismatch' });
  });
  port.onDisconnect.addListener(() => {
    void chrome.runtime.lastError;
    if (nativePort !== port) return;
    nativePort = undefined;
    diagnostics = undefined;
    diagnosticsPending = undefined;
    for (const callback of pending.values()) callback({ error: 'helper_unavailable' });
    scheduleWatch();
  });
  return port;
}

function nativeRequest(command, fields = {}) {
  return new Promise((resolve) => {
    const id = crypto.randomUUID();
    const timeoutMs = ['chooseCookies', 'chooseDirectory'].includes(command) ? 150_000 : command === 'inspect' ? 150_000 : 45_000;
    const timeout = setTimeout(() => finish({ error: 'helper_timeout' }), timeoutMs);

    function finish(reply) {
      clearTimeout(timeout);
      pending.delete(id);
      resolve(reply);
    }

    pending.set(id, finish);
    try {
      connect().postMessage({ id, protocolVersion: PROTOCOL_VERSION, command, ...fields });
    } catch {
      finish({ error: 'helper_unavailable' });
    }
  });
}

function getDiagnostics(force) {
  if (!force && diagnostics?.expires > Date.now()) return diagnostics.reply;
  if (diagnosticsPending) return diagnosticsPending;
  const pendingRequest = nativeRequest('hello').then(reply => {
    if (diagnosticsPending === pendingRequest) {
      diagnosticsPending = undefined;
      diagnostics = reply.error ? undefined : { reply, expires: Date.now() + 120_000 };
    }
    return reply;
  });
  diagnosticsPending = pendingRequest;
  return pendingRequest;
}

function updateJob(reply, url, optionId, started = false) {
  if (!reply.job?.id) return reply;
  const write = jobWrite.then(async () => {
    const { lastJob } = await chrome.storage.local.get('lastJob');
    if (!started && (lastJob?.id !== reply.job.id || !activeStates.has(lastJob.state))) {
      return { job: lastJob };
    }
    const job = { url, optionId, ...reply.job };
    if (started || JSON.stringify(lastJob) !== JSON.stringify(job)) {
      await chrome.storage.local.set({ lastJob: job });
    }
    return { ...reply, job };
  });
  jobWrite = write.catch(() => {});
  return write;
}

async function readJob(command, lastJob) {
  const reply = await nativeRequest(command, { jobId: lastJob.id });
  if (['job_not_found', 'helper_unavailable'].includes(reply.error)) {
    return updateJob({ job: { ...lastJob, state: 'interrupted' } }, lastJob.url, lastJob.optionId);
  }
  return updateJob(reply, lastJob.url, lastJob.optionId);
}

function scheduleWatch() {
  if (watchTimer || watching) return;
  watchTimer = setTimeout(async () => {
    watchTimer = undefined;
    watching = true;
    let again = false;
    try {
      const { lastJob } = await chrome.storage.local.get('lastJob');
      if (activeStates.has(lastJob?.state)) {
        const reply = await readJob('status', lastJob);
        again = !!reply.error || activeStates.has(reply.job?.state);
      }
    } finally {
      watching = false;
      if (again) scheduleWatch();
    }
  }, 1000);
}

async function youtubeCookies(tabId) {
  if (!Number.isInteger(tabId)) return '';
  const stores = await chrome.cookies.getAllCookieStores();
  const store = stores.find(({ tabIds }) => tabIds.includes(tabId));
  if (!store) return '';
  const cookies = await chrome.cookies.getAll({ domain: 'youtube.com', storeId: store.id });
  const lines = ['# Netscape HTTP Cookie File'];
  for (const cookie of cookies) {
    const domain = cookie.domain.replace(/^\./, '').toLowerCase();
    if (domain !== 'youtube.com' && !domain.endsWith('.youtube.com')) continue;
    const scope = cookie.hostOnly ? domain : '.' + domain;
    const fields = [
      (cookie.httpOnly ? '#HttpOnly_' : '') + scope, cookie.hostOnly ? 'FALSE' : 'TRUE',
      cookie.path, cookie.secure ? 'TRUE' : 'FALSE', Math.floor(cookie.expirationDate ?? 0),
      cookie.name, cookie.value,
    ];
    if (fields.some(field => /[\t\r\n\0]/.test(String(field)))) continue;
    lines.push(fields.join('\t'));
  }
  if (lines.length === 1) return '';
  const result = lines.join('\n') + '\n';
  return new TextEncoder().encode(result).length <= 384 * 1024 ? result : '';
}

async function browserSession(tabId) {
  try {
    return await youtubeCookies(tabId);
  } catch {
    return '';
  }
}

async function inspectVideo(url, tabId, useCookies, force) {
  const initialCookies = useCookies ? await browserSession(tabId) : '';
  const reply = await nativeRequest('inspect', initialCookies ? { url, cookies: initialCookies, force } : { url, force });
  if (initialCookies || !['login_required', 'cookies_expired'].includes(reply.error)) return reply;
  const cookies = await browserSession(tabId);
  if (!cookies) return reply;
  const retried = await nativeRequest('inspect', { url, cookies });
  return retried.error === 'unknown_command' || retried.error === 'cookies_invalid' ? reply : retried;
}

async function handle(message) {
  if (message.command === 'hello') return getDiagnostics(message.force === true);
  if (message.command === 'chooseDirectory') {
    const { downloadDirectory } = await chrome.storage.local.get('downloadDirectory');
    const reply = await nativeRequest('chooseDirectory', { directory: downloadDirectory });
    if (reply.directory) await chrome.storage.local.set({ downloadDirectory: reply.directory });
    return reply;
  }
  if (message.command === 'resetDirectory') {
    await chrome.storage.local.remove('downloadDirectory');
    return {};
  }
  if (message.command === 'start') {
    const { url, optionId } = message;
    const { downloadDirectory } = await chrome.storage.local.get('downloadDirectory');
    const reply = await updateJob(await nativeRequest('start', { url, optionId, directory: downloadDirectory }), url, optionId, true);
    if (reply.job) scheduleWatch();
    return reply;
  }
  if (message.command === 'inspect') return inspectVideo(message.url, message.tabId, message.useCookies === true, message.force === true);
  if (['restore', 'status', 'cancel'].includes(message.command)) {
    const { lastJob } = await chrome.storage.local.get('lastJob');
    if (!lastJob || !activeStates.has(lastJob.state)) return { job: lastJob };
    if (message.command === 'status') return { job: lastJob };
    const reply = await readJob(message.command === 'cancel' ? 'cancel' : 'status', lastJob);
    if (activeStates.has(reply.job?.state)) scheduleWatch();
    return reply;
  }
  return nativeRequest(message.command);
}

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (!commands.has(message?.command)) return;
  if (sender.url !== chrome.runtime.getURL('popup.html')) {
    if (sender.frameId !== 0 || !sender.url?.startsWith('https://www.youtube.com/') || !sender.tab) return;
    const page = new URL(sender.url);
    const video = page.pathname === '/watch' ? page.searchParams.get('v') : /^\/shorts\/([\w-]{11})$/.exec(page.pathname)?.[1];
    if (!/^[\w-]{11}$/.test(video ?? '')) return;
    message = { ...message, url: 'https://www.youtube.com/watch?v=' + video, tabId: sender.tab.id };
  }
  handle(message).then(sendResponse, () => sendResponse({ error: 'extension_error' }));
  return true;
});
