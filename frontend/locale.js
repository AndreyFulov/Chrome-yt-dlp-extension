const appName = 'chrome yt-dlp extension';
const repositoryURL = 'https://github.com/AndreyFulov/Chrome-yt-dlp-extension';

const panelText = {
  en: {
    settings: 'Settings', language: 'Language', connecting: 'Connecting…', formats: 'Finding formats…',
    retry: 'Retry', cookies: 'Choose cookies.txt', format: 'Format', quality: 'Quality',
    downloads: 'Downloads folder', chooseDirectory: 'Choose folder…', resetDirectory: 'Use default',
    download: 'Download', job: 'Download status', cancel: 'Cancel', retryJob: 'Retry download',
    diagnostics: 'Helper components', video: 'YouTube video', saved: 'Saved: {path}',
    done: 'File saved.', downloading: 'Downloading…', processing: 'Processing…',
    cancelled: 'Download cancelled.', interrupted: 'Download interrupted.',
    unknownState: 'Unknown download status.', missing: 'Missing', unavailable: 'Cannot start',
    unverified: 'Unchecked', available: 'Available', checkTools: 'Check the installed components: {tools}.',
    tabError: 'Could not read the current tab address.', tryAgain: 'Something went wrong. Try again.',
    rateDelay: ' Wait {seconds} s.', github: 'View on GitHub',
  },
  ru: {
    settings: 'Настройки', language: 'Язык', connecting: 'Проверяем соединение…', formats: 'Получаем список форматов…',
    retry: 'Повторить', cookies: 'Выбрать cookies.txt', format: 'Формат', quality: 'Качество',
    downloads: 'Папка «Загрузки»', chooseDirectory: 'Выбрать папку…', resetDirectory: 'По умолчанию',
    download: 'Скачать', job: 'Загрузка', cancel: 'Отменить', retryJob: 'Повторить загрузку',
    diagnostics: 'Компоненты помощника', video: 'Ролик YouTube', saved: 'Сохранено: {path}',
    done: 'Файл сохранён.', downloading: 'Загрузка…', processing: 'Обработка файла…',
    cancelled: 'Загрузка отменена.', interrupted: 'Загрузка прервана.',
    unknownState: 'Неизвестное состояние задачи.', missing: 'Не найден', unavailable: 'Не запускается',
    unverified: 'Не проверен', available: 'Доступен', checkTools: 'Проверьте установку компонентов: {tools}.',
    tabError: 'Не удалось прочитать адрес активной вкладки.', tryAgain: 'Операция не удалась. Повторите попытку.',
    rateDelay: ' Подождите {seconds} с.', github: 'Открыть GitHub',
  },
};

const englishErrors = {
  helper_unavailable: 'The helper is unavailable. Install or reinstall it.',
  helper_timeout: 'The helper did not respond. Try again.',
  protocol_mismatch: 'Update the extension and helper to compatible versions.',
  invalid_response: 'The helper returned an unexpected response. Check its version.',
  missing_tool: 'Install the missing helper components.',
  invalid_url: 'Open a YouTube video or Short.',
  login_required: 'This video requires a sign-in. Choose cookies.txt.',
  cookies_expired: 'The session may have expired. Choose a fresh cookies.txt.',
  cookies_unreadable: 'The cookies.txt file could not be read. Choose it again.',
  cookies_invalid: 'Choose a readable cookies.txt file in Netscape format.',
  cookies_cancelled: 'File selection cancelled.',
  cookies_dialog_failed: 'Could not open the file picker.',
  access_denied: 'The current session cannot access this video.',
  video_unavailable: 'The video was removed or is unavailable.',
  rate_limited: 'YouTube is limiting requests. Try again later.',
  http_403: 'YouTube rejected the request (HTTP 403).',
  network_error: 'Check your internet connection and try again.',
  timeout: 'YouTube took too long to respond. Try again.',
  no_formats: 'No supported formats are available for this video.',
  format_unavailable: 'The selected quality is unavailable. Refreshing the list.',
  disk_full: 'Free up disk space and try again.',
  write_failed: 'Check your permissions for the download folder.',
  directory_unavailable: 'Choose another folder or use the default Downloads folder.',
  directory_dialog_failed: 'Could not open the folder picker.',
  ffmpeg_failed: 'FFmpeg could not process the tracks.',
  media_invalid: 'A downloaded track was incomplete or corrupt. Try again.',
  encoder_unavailable: 'FFmpeg lacks an encoder for the selected format.',
  ffmpeg_mux_failed: 'FFmpeg could not combine the tracks. Try another quality.',
  extract_failed: 'Could not find formats. Check the yt-dlp, EJS, and Deno versions below.',
  download_failed: 'Could not download the video. Try again.',
  busy: 'The helper is busy. Wait for the current operation to finish.',
};

function panelLanguage(value) {
  if (value === 'ru' || value === 'en') return value;
  return /^ru(?:-|$)/i.test(chrome.i18n?.getUILanguage() ?? navigator.language) ? 'ru' : 'en';
}

function translatePanel(language, key, values = {}) {
  return panelText[language][key].replace(/\{(\w+)\}/g, (_, name) => String(values[name] ?? ''));
}

function addPanelFooter(parent) {
  const footer = document.createElement('footer');
  const link = document.createElement('a');
  link.href = repositoryURL;
  link.target = '_blank';
  link.rel = 'noopener noreferrer';
  const ns = 'http://www.w3.org/2000/svg';
  const icon = document.createElementNS(ns, 'svg');
  icon.setAttribute('viewBox', '0 0 24 24');
  icon.setAttribute('aria-hidden', 'true');
  const path = document.createElementNS(ns, 'path');
  path.setAttribute('fill', 'currentColor');
  path.setAttribute('d', 'M12 0C5.37 0 0 5.37 0 12c0 5.3 3.44 9.8 8.21 11.39.6.11.82-.26.82-.58v-2.23c-3.34.73-4.04-1.42-4.04-1.42-.55-1.39-1.33-1.76-1.33-1.76-1.09-.74.08-.73.08-.73 1.2.09 1.84 1.24 1.84 1.24 1.07 1.83 2.81 1.3 3.49.99.11-.78.42-1.3.76-1.6-2.67-.3-5.47-1.33-5.47-5.93 0-1.31.47-2.38 1.24-3.22-.12-.3-.54-1.52.12-3.17 0 0 1.01-.32 3.3 1.23.96-.27 1.98-.4 3-.4 1.02 0 2.04.13 3 .4 2.29-1.55 3.3-1.23 3.3-1.23.66 1.65.24 2.87.12 3.17.77.84 1.24 1.91 1.24 3.22 0 4.61-2.81 5.62-5.49 5.92.43.38.82 1.1.82 2.22v3.29c0 .32.22.69.83.57C20.56 21.79 24 17.3 24 12 24 5.37 18.63 0 12 0Z');
  icon.append(path);
  const label = document.createElement('span');
  label.dataset.text = 'github';
  link.append(icon, label);
  footer.append(link);
  parent.append(footer);
}
