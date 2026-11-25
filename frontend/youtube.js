(() => {
  const button = document.createElement('button');
  button.type = 'button';
  button.className = 'ytdlp-extension-download';
  button.title = appName;
  button.setAttribute('aria-expanded', 'false');
  button.setAttribute('aria-controls', 'ytdlp-extension-menu');

  const menu = document.createElement('div');
  menu.id = 'ytdlp-extension-menu';
  menu.className = 'ytdlp-extension-menu';
  menu.popover = 'auto';
  menu.setAttribute('aria-label', appName);
  function localizeButton(value) {
    button.textContent = translatePanel(panelLanguage(value), 'download');
  }
  localizeButton();
  chrome.storage.local.get('interfaceLanguage').then(({ interfaceLanguage }) => localizeButton(interfaceLanguage), () => {});
  chrome.storage.onChanged.addListener((changes, area) => {
    if (area === 'local' && changes.interfaceLanguage) localizeButton(changes.interfaceLanguage.newValue);
  });
  const shadow = menu.attachShadow({ mode: 'closed' });
  const styles = new CSSStyleSheet();
  styles.replaceSync(`
    :host { font: 14px/1.45 system-ui, sans-serif; color-scheme: light dark; }
    main { padding: 18px; }
    h1 { font-size: 18px; font-weight: 700; margin: 0 0 12px; letter-spacing: -.02em; }
    h2, p, dl { margin: 0 0 10px; overflow-wrap: anywhere; }
    h2 { font-size: 15px; }
    form, section, details { margin-top: 12px; }
    .selectors { display: grid; grid-template-columns: 1fr 1.6fr; gap: 10px; margin-bottom: 12px; }
    .destination { margin-bottom: 10px; }
    label, label select, #download, progress { display: block; width: 100%; }
    select, button { box-sizing: border-box; padding: 7px 10px;
      border: 1px solid color-mix(in srgb, currentColor 20%, transparent);
      border-radius: 10px; font: inherit; color: inherit; background: var(--yt-spec-badge-chip-background, Canvas); }
    select { margin-top: 4px; }
    button { cursor: pointer; margin: 0 5px 4px 0; }
    button:disabled { cursor: default; opacity: .6; }
    #download { background: #bd1332; color: white; border-color: #bd1332; font-weight: 650; }
    #download:hover:enabled { background: #97112b; }
    footer { margin-top: 16px; padding-top: 12px; border-top: 1px solid color-mix(in srgb, currentColor 18%, transparent); }
    footer a { display: inline-flex; align-items: center; gap: 7px; color: inherit; text-decoration: none; }
    footer a:hover { text-decoration: underline; }
    footer svg { width: 16px; height: 16px; }
    dl { display: grid; grid-template-columns: max-content 1fr; gap: 2px 8px; margin-top: 8px; }
    dd { margin: 0; overflow-wrap: anywhere; }
    summary { cursor: pointer; }
    [hidden] { display: none !important; }
  `);
  shadow.adoptedStyleSheets = [styles];

  let menuSession;
  let menuVideo;
  function element(parent, tag, id, key) {
    const node = document.createElement(tag);
    if (id) node.id = id;
    if (key) node.dataset.text = key;
    parent.append(node);
    return node;
  }

  function action(parent, id, label) {
    const node = element(parent, 'button', id, label);
    node.type = 'button';
    node.hidden = true;
  }

  function buildMenu() {
    shadow.replaceChildren();
    const main = element(shadow, 'main');
    element(main, 'h1').textContent = appName;
    element(main, 'p', 'page');
    const status = element(main, 'p', 'status');
    status.setAttribute('role', 'status');
    action(main, 'retry', 'retry');
    action(main, 'cookies', 'cookies');
    const form = element(main, 'form', 'download-form');
    form.hidden = true;
    const selectors = element(form, 'div');
    selectors.className = 'selectors';
    for (const field of ['format', 'quality']) {
      const label = element(selectors, 'label');
      element(label, 'span', '', field);
      element(label, 'select', field);
    }
    const destination = element(form, 'div');
    destination.className = 'destination';
    element(destination, 'p', 'destination').setAttribute('aria-live', 'polite');
    element(destination, 'button', 'choose-directory', 'chooseDirectory').type = 'button';
    action(destination, 'reset-directory', 'resetDirectory');
    element(form, 'button', 'download', 'download').type = 'submit';
    const job = element(main, 'section', 'job');
    job.hidden = true;
    element(job, 'h2', '', 'job');
    element(job, 'progress', 'progress').max = 100;
    element(job, 'p', 'job-status').setAttribute('role', 'status');
    action(job, 'cancel', 'cancel');
    action(job, 'retry-job', 'retryJob');
    const diagnostics = element(main, 'details', 'diagnostics');
    diagnostics.hidden = true;
    element(diagnostics, 'summary', '', 'diagnostics');
    element(diagnostics, 'dl', 'tools');
  }

  function positionMenu() {
    if (!menu.matches(':popover-open')) return;
    const anchor = button.getBoundingClientRect();
    if (!button.isConnected || anchor.bottom < 0 || anchor.top > innerHeight) {
      menu.hidePopover();
      return;
    }
    const below = innerHeight - anchor.bottom - 16;
    const above = anchor.top - 16;
    const openAbove = below < 230 && above > below;
    menu.style.maxHeight = Math.max(65, openAbove ? above : below) + 'px';
    const box = menu.getBoundingClientRect();
    menu.style.left = Math.max(8, Math.min(anchor.left, innerWidth - box.width - 8)) + 'px';
    menu.style.top = (openAbove ? Math.max(8, anchor.top - box.height - 8) : anchor.bottom + 8) + 'px';
  }

  function closeMenu() {
    if (menu.matches(':popover-open')) menu.hidePopover();
  }

  menu.addEventListener('toggle', () => {
    const open = menu.matches(':popover-open');
    button.setAttribute('aria-expanded', String(open));
    if (!open) {
      menuSession?.abort();
      menuSession = undefined;
    }
  });

  button.addEventListener('click', (event) => {
    if (!event.isTrusted) return;
    event.preventDefault();
    if (menu.matches(':popover-open')) {
      closeMenu();
      return;
    }
    if (!menu.isConnected) document.body.append(menu);
    menuSession?.abort();
    menuSession = new AbortController();
    menuVideo = location.pathname + location.search;
    buildMenu();
    menu.showPopover({ source: button });
    positionMenu();
    mountDownloadPanel(shadow, { embedded: true, getUrl: () => location.href, signal: menuSession.signal });
  });
  new ResizeObserver(positionMenu).observe(menu);
  window.addEventListener('resize', positionMenu, { passive: true });
  window.addEventListener('scroll', positionMenu, { passive: true, capture: true });

  function insertDownloadButton() {
    const url = new URL(location.href);
    if (menuVideo !== url.pathname + url.search) closeMenu();
    let bar;
    if (url.pathname === '/watch' && /^[\w-]{11}$/.test(url.searchParams.get('v') ?? '')) {
      bar = document.querySelector('ytd-watch-metadata #top-level-buttons-computed, #above-the-fold #top-level-buttons-computed');
    } else if (/^\/shorts\/[\w-]{11}$/.test(url.pathname)) {
      bar = document.querySelector('ytd-reel-video-renderer[is-active] #actions, yt-shorts-video-renderer[is-active] #actions');
    }
    if (!bar) {
      closeMenu();
      button.remove();
      return;
    }
    let like = bar.querySelector('segmented-like-dislike-button-view-model, ytd-segmented-like-dislike-button-renderer, #like-button');
    while (like && like.parentElement !== bar) like = like.parentElement;
    if (like) {
      if (button.nextElementSibling !== like) like.before(button);
    } else if (button.parentElement !== bar) {
      bar.prepend(button);
    }
  }

  let scheduled = false;
  function schedule() {
    if (scheduled) return;
    scheduled = true;
    requestAnimationFrame(() => {
      scheduled = false;
      insertDownloadButton();
    });
  }

  document.addEventListener('yt-navigate-finish', schedule);
  new MutationObserver(schedule).observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['is-active'] });
  schedule();
})();
