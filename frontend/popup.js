function mountDownloadPanel(root, { embedded = false, getUrl, signal } = {}) {
  const byId = (id) => root.querySelector('#' + id);
  const page = byId('page');
  const status = byId('status');
  const retry = byId('retry');
  const cookies = byId('cookies');
  const form = byId('download-form');
  const format = byId('format');
  const quality = byId('quality');
  const download = byId('download');
  const cancel = byId('cancel');
  const retryJob = byId('retry-job');
  const destination = byId('destination');
  const chooseDirectory = byId('choose-directory');
  const resetDirectory = byId('reset-directory');
  const languageControl = byId('language-select');
  const activeStates = new Set(['downloading', 'processing']);
  const panel = root.querySelector('main');
  addPanelFooter(panel);
  let language = panelLanguage();
  const t = (key, values) => translatePanel(language, key, values);
  let statusRenderer = () => t('connecting');
  let pageRenderer = () => '';
  let currentDirectory;
  let currentTools;

  function setStatus(key, values) {
    statusRenderer = () => key ? t(key, values) : '';
    status.textContent = statusRenderer();
  }

  function applyLanguage(value) {
    language = panelLanguage(value);
    panel.lang = language;
    for (const item of panel.querySelectorAll('[data-text]')) {
      item.textContent = t(item.dataset.text);
    }
    if (languageControl) languageControl.value = language;
    page.textContent = pageRenderer();
    status.textContent = statusRenderer();
    renderDestination(currentDirectory);
    if (currentTools) showTools(currentTools);
    if (job) renderJob(job);
    if (options.length) renderQuality(quality.value);
  }

  const messages = {
    helper_unavailable: 'Помощник недоступен. Установите или переустановите его.',
    helper_timeout: 'Помощник не ответил. Повторите попытку.',
    protocol_mismatch: 'Обновите расширение и помощник до совместимых версий.',
    invalid_response: 'Помощник вернул непонятный ответ. Проверьте его версию.',
    missing_tool: 'Установите недостающие компоненты помощника.',
    invalid_url: 'Откройте ролик или Shorts на YouTube.',
    login_required: 'Для этого ролика нужен вход. Выберите cookies.txt.',
    cookies_expired: 'Сессия, вероятно, истекла. Выберите свежий cookies.txt.',
    cookies_unreadable: 'Файл cookies.txt недоступен. Выберите его снова.',
    cookies_invalid: 'Выберите читаемый cookies.txt в формате Netscape.',
    cookies_cancelled: 'Выбор файла отменён.',
    cookies_dialog_failed: 'Не удалось открыть системный диалог выбора файла.',
    access_denied: 'У текущей сессии нет доступа к ролику.',
    video_unavailable: 'Ролик недоступен или удалён.',
    rate_limited: 'YouTube ограничил число запросов. Повторите попытку позже.',
    http_403: 'YouTube отклонил запрос (HTTP 403).',
    network_error: 'Проверьте интернет-соединение и повторите попытку.',
    timeout: 'YouTube не ответил вовремя. Повторите попытку.',
    no_formats: 'Подходящих форматов для этого ролика нет.',
    format_unavailable: 'Выбранное качество больше недоступно. Обновляем список.',
    disk_full: 'Освободите место на диске и повторите попытку.',
    write_failed: 'Проверьте разрешения на запись в папку загрузок.',
    directory_unavailable: 'Выбранная папка недоступна. Укажите другую папку или верните папку «Загрузки».',
    directory_dialog_failed: 'Не удалось открыть окно выбора папки.',
    ffmpeg_failed: 'FFmpeg завершился с ошибкой при обработке дорожек.',
    media_invalid: 'Полученная дорожка повреждена или неполная. Повторите загрузку.',
    encoder_unavailable: 'В установленном FFmpeg отсутствует кодировщик для выбранного формата.',
    ffmpeg_mux_failed: 'FFmpeg не смог записать дорожки в выбранный формат. Попробуйте другой вариант качества.',
    extract_failed: 'Не удалось получить форматы. Проверьте версии yt-dlp, EJS и Deno ниже.',
    download_failed: 'Не удалось скачать ролик. Повторите попытку.',
    busy: 'Помощник занят. Дождитесь завершения операции.',
  };

  const cookieErrors = new Set([
    'login_required', 'cookies_expired', 'access_denied', 'cookies_unreadable', 'cookies_invalid',
    'cookies_cancelled', 'cookies_dialog_failed',
  ]);
  let activeUrl;
  let videoUrl;
  let options = [];
  let job;
  let busy = false;

  function currentVideo(rawUrl) {
    try {
      const url = new URL(rawUrl);
      if (url.protocol !== 'https:' || url.username || url.password || url.port) return null;
      const segments = url.pathname.split('/');
      let id;
      if (url.hostname === 'youtu.be' && segments.length === 2) id = segments[1];
      if (['youtube.com', 'www.youtube.com', 'm.youtube.com'].includes(url.hostname)) {
        if (url.pathname === '/watch') id = url.searchParams.get('v');
        if (segments[1] === 'shorts' && segments.length === 3) id = segments[2];
      }
      return /^[\w-]{11}$/.test(id ?? '') ? 'https://www.youtube.com/watch?v=' + id : null;
    } catch {
      return null;
    }
  }

  async function request(command, fields = {}) {
    if (signal?.aborted) return { error: 'cancelled' };
    try {
      return await chrome.runtime.sendMessage({ command, ...fields }) ?? { error: 'invalid_response' };
    } catch {
      return { error: 'helper_unavailable' };
    }
  }

  function errorText(error, retryAfter) {
    const message = (language === 'ru' ? messages : englishErrors)[error] ?? t('tryAgain');
    return error === 'rate_limited' && Number.isFinite(retryAfter) && retryAfter > 0
      ? message + t('rateDelay', { seconds: Math.ceil(retryAfter) })
      : message;
  }

  function showError(reply) {
    statusRenderer = () => errorText(reply.error, reply.retryAfter);
    status.textContent = statusRenderer();
    cookies.hidden = !cookieErrors.has(reply.error);
    retry.hidden = false;
  }

  function showTools(tools) {
    currentTools = tools;
    const list = byId('tools');
    list.replaceChildren();
    for (const [name, info] of Object.entries(tools ?? {})) {
      const label = document.createElement('dt');
      const value = document.createElement('dd');
      label.textContent = name;
      value.textContent = info.version || t(Object.hasOwn(panelText[language], info.status) ? info.status : 'available');
      list.append(label, value);
    }
    byId('diagnostics').hidden = !list.childElementCount;
  }

  function renderQuality(preferred) {
    const choices = options.filter((option) => option.format === format.value);
    quality.replaceChildren(...choices.map((option) => new Option(
      language === 'en' ? option.label.replace('кбит/с', 'kbps') : option.label, option.id,
    )));
    if (choices.some((option) => option.id === preferred)) quality.value = preferred;
  }

  async function inspect(url, preferred, useCookies = false, force = false) {
    videoUrl = url;
    form.hidden = true;
    retry.hidden = true;
    cookies.hidden = true;
    setStatus('formats');
    const [tab] = embedded ? [] : await chrome.tabs.query({ active: true, currentWindow: true });
    const reply = await request('inspect', { url, tabId: tab?.id, useCookies, force });
    if (reply.error) {
      showError(reply);
      return;
    }
    options = Array.isArray(reply.options) ? reply.options.filter((option) =>
      typeof option?.id === 'string' && typeof option.label === 'string'
        && ['mp4', 'm4a', 'mp3'].includes(option.format)) : [];
    pageRenderer = () => reply.title || t('video');
    page.textContent = pageRenderer();
    if (!options.length) {
      showError({ error: 'no_formats' });
      return;
    }
    const formats = [...new Set(options.map((option) => option.format))];
    format.replaceChildren(...formats.map((name) => new Option(name.toUpperCase(), name)));
    const previous = options.find((option) => option.id === preferred);
    if (previous) format.value = previous.format;
    renderQuality(preferred);
    form.hidden = false;
    setStatus();
  }

  function renderJob(nextJob) {
    if (!nextJob?.id) return;
    job = nextJob;
    const active = activeStates.has(job.state);
    byId('job').hidden = false;
    const progress = byId('progress');
    progress.hidden = !active;
    if (active && Number.isFinite(job.percent)) progress.value = Math.max(0, Math.min(job.percent, 100));
    else progress.removeAttribute('value');

    const labels = {
      downloading: t('downloading'),
      processing: t('processing'),
      done: !embedded && job.path ? t('saved', { path: job.path }) : t('done'),
      cancelled: t('cancelled'),
      interrupted: t('interrupted'),
      failed: errorText(job.error, job.retryAfter),
    };
    const text = labels[job.state] ?? t('unknownState');
    const jobStatus = byId('job-status');
    const summary = active && Number.isFinite(job.percent) ? text + ' ' + Math.round(progress.value) + '%.' : text;
    if (jobStatus.textContent !== summary) jobStatus.textContent = summary;
    cancel.hidden = !active;
    const matchingVideo = !embedded || job.url === activeUrl;
    retryJob.hidden = !matchingVideo || !job.url || !['interrupted', 'failed', 'cancelled'].includes(job.state);
    if (matchingVideo && job.state === 'failed' && job.url && cookieErrors.has(job.error)) {
      videoUrl = job.url;
      cookies.hidden = false;
    }
    download.disabled = busy || active;
  }

  async function act(work) {
    if (busy || signal?.aborted) return;
    busy = true;
    const controls = [retry, cookies, cancel, retryJob, chooseDirectory, resetDirectory];
    for (const button of [...controls, download]) button.disabled = true;
    try {
      await work();
    } finally {
      busy = false;
      for (const button of controls) button.disabled = false;
      download.disabled = activeStates.has(job?.state);
    }
  }

  async function refresh(force = false) {
    const hello = await request('hello', { force });
    if (hello.error) {
      form.hidden = true;
      showError(hello);
      return;
    }
    showTools(hello.tools);
    const unavailable = Object.entries(hello.tools ?? {})
      .filter(([, info]) => ['missing', 'unavailable'].includes(info.status))
      .map(([name]) => name);
    if (unavailable.length) {
      byId('diagnostics').open = true;
      setStatus('checkTools', { tools: unavailable.join(', ') });
      retry.hidden = false;
      form.hidden = true;
      return;
    }
    if (activeStates.has(job?.state)) {
      form.hidden = true;
      setStatus();
      return;
    }
    if (videoUrl || activeUrl) await inspect(videoUrl || activeUrl, undefined, false, force);
    else {
      retry.hidden = true;
      statusRenderer = () => errorText('invalid_url');
      status.textContent = statusRenderer();
    }
  }

  format.addEventListener('change', () => renderQuality());
  retry.addEventListener('click', () => act(() => refresh(true)));
  cookies.addEventListener('click', () => act(async () => {
    const reply = await request('chooseCookies');
    if (reply.error) showError(reply);
    else await inspect(videoUrl);
  }));
  form.addEventListener('submit', (event) => {
    event.preventDefault();
    act(startDownload);
  });

  function renderDestination(directory) {
    currentDirectory = directory;
    destination.textContent = directory || t('downloads');
    resetDirectory.hidden = !directory;
  }

  chooseDirectory.addEventListener('click', () => act(async () => {
    const reply = await request('chooseDirectory');
    if (reply.error) showError(reply);
    else if (reply.directory) {
      renderDestination(reply.directory);
      setStatus();
    }
  }));
  resetDirectory.addEventListener('click', () => act(async () => {
    const reply = await request('resetDirectory');
    if (reply.error) showError(reply);
    else {
      renderDestination('');
      setStatus();
    }
  }));

  async function startDownload() {
    if (activeStates.has(job?.state) || !quality.value) return;
    const optionId = quality.value;
    const reply = await request('start', { url: videoUrl, optionId });
    if (reply.error === 'format_unavailable') await inspect(videoUrl);
    else if (reply.error) showError(reply);
    else if (reply.job?.id) {
      setStatus();
      renderJob({ url: videoUrl, optionId, ...reply.job });
    } else {
      showError({ error: 'invalid_response' });
    }
  }
  cancel.addEventListener('click', () => act(async () => {
    const reply = await request('cancel');
    if (reply.error) showError(reply);
    else renderJob(reply.job && { url: job.url, optionId: job.optionId, ...reply.job });
  }));
  retryJob.addEventListener('click', () => act(async () => {
    if (!job?.url) return;
    await inspect(job.url, job.optionId, ['login_required', 'cookies_expired'].includes(job.error));
    if (!form.hidden && quality.value === job.optionId) await startDownload();
  }));

  function onJobChanged(changes, area) {
    if (area !== 'local') return;
    if (changes.interfaceLanguage) applyLanguage(changes.interfaceLanguage.newValue);
    if (changes.downloadDirectory) renderDestination(changes.downloadDirectory.newValue);
    if (!changes.lastJob?.newValue) return;
    const wasActive = activeStates.has(job?.state);
    renderJob(changes.lastJob.newValue);
    if (job.state === 'failed' && job.error === 'format_unavailable' && job.url === videoUrl) {
      act(() => inspect(videoUrl));
    } else if (wasActive && job.state === 'done' && form.hidden && activeUrl) {
      act(() => inspect(activeUrl));
    }
  }

  chrome.storage.onChanged.addListener(onJobChanged);
  signal?.addEventListener('abort', () => chrome.storage.onChanged.removeListener(onJobChanged), { once: true });
  languageControl?.addEventListener('change', async () => {
    const previous = language;
    try {
      await chrome.storage.local.set({ interfaceLanguage: languageControl.value });
      applyLanguage(languageControl.value);
    } catch {
      applyLanguage(previous);
      showError({});
    }
  });
  applyLanguage();

  act(async () => {
    const preferences = chrome.storage.local.get(['downloadDirectory', 'interfaceLanguage'])
      .then(({ downloadDirectory, interfaceLanguage }) => {
        renderDestination(downloadDirectory);
        applyLanguage(interfaceLanguage);
      }, () => {});
    try {
      if (embedded) {
        activeUrl = currentVideo(getUrl());
      } else {
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        activeUrl = currentVideo(tab?.url);
      }
    } catch {
      pageRenderer = () => t('tabError');
      page.textContent = pageRenderer();
    }
    const restored = await request('restore');
    if (!restored.error) renderJob(restored.job);
    await Promise.all([refresh(), preferences]);
    if (job?.state === 'failed' && cookieErrors.has(job.error)) renderJob(job);
  });
}

if (location.protocol === 'chrome-extension:') mountDownloadPanel(document);
