// State
let currentView = 'dashboard';
let activeServer = null;
let currentFilesPath = '';
let currentDbDriver = 'postgres';
let currentDbSubTab = 'kv';
let currentScriptLang = 'python';
let currentEditingPath = '';
let scriptLogSource = null;

// Spinner & Connection state timer
let offlineTimeout = null;

// Direct vs Proxy mode detection
const isDirectMode = window.location.protocol === 'file:' || 
  (!window.location.host.includes('localhost') && !window.location.host.includes('127.0.0.1') && !window.location.port);

// ================= THEMES =================
function initTheme() {
  const saved = localStorage.getItem('node_theme') || 'dark';
  setTheme(saved, false);
}

function setTheme(themeName, save = true) {
  document.documentElement.setAttribute('data-theme', themeName);
  document.body.setAttribute('data-theme', themeName);
  if (save) {
    localStorage.setItem('node_theme', themeName);
    closeModal('modalTheme');
  }
}

function openThemeModal() {
  openModal('modalTheme');
}

function triggerFileUpload() {
  const input = document.getElementById('fileUploadInput');
  if (input) {
    input.value = '';
    input.click();
  }
}

// Initialize
document.addEventListener('DOMContentLoaded', () => {
  initTheme();
  initServers();
  // Auto-refresh active server ping every 5 seconds
  setInterval(refreshHealthBanner, 5000);
});

// Navigation
function navigateTo(view) {
  currentView = view;
  document.getElementById('viewDashboard').style.display = view === 'dashboard' ? 'block' : 'none';
  document.getElementById('viewFiles').style.display = view === 'files' ? 'block' : 'none';
  document.getElementById('viewDb').style.display = view === 'db' ? 'block' : 'none';
  document.getElementById('viewScripts').style.display = view === 'scripts' ? 'block' : 'none';

  const brand = document.getElementById('headerBrand');
  const backBtn = document.getElementById('btnBack');
  const backBtnText = document.getElementById('backBtnText');

  if (view === 'dashboard') {
    if (brand) brand.style.display = 'flex';
    if (backBtn) backBtn.style.display = 'none';
    refreshHealthBanner();
  } else {
    // "когда я зашел в какой то раздел, назвение уберается"
    if (brand) brand.style.display = 'none';
    if (backBtn) backBtn.style.display = 'inline-flex';

    if (view === 'files') {
      if (backBtnText) backBtnText.innerText = 'Файлы';
      loadFiles(currentFilesPath);
    } else if (view === 'db') {
      if (backBtnText) backBtnText.innerText = 'Базы данных';
      loadDatabasesList();
    } else if (view === 'scripts') {
      if (backBtnText) backBtnText.innerText = 'Скрипты';
      loadCurrentScript();
      loadScriptsList();
      const curId = document.getElementById('scriptIdInput').value.trim() || 'main';
      loadScriptSnapshots(curId);
      connectScriptWS(curId);
    }
  }
}

function refreshCurrentView() {
  if (currentView === 'dashboard') {
    refreshHealthBanner();
  } else if (currentView === 'files') {
    loadFiles(currentFilesPath);
  } else if (currentView === 'scripts') {
    const curId = document.getElementById('scriptIdInput').value.trim() || 'main';
    connectScriptWS(curId);
    loadScriptsList();
    loadScriptSnapshots(curId);
  } else if (currentView === 'db') {
    loadDatabasesList();
  }
}

// ================= TOP SPINNER & HEALTH LOGIC =================
// "когда подключился к серверу этот круг пропадает, а когда подключения нету 10 секунд то появляется"

function setConnectedState(isConnected) {
  const topSpinner = document.getElementById('topSpinner');

  if (isConnected) {
    if (offlineTimeout) {
      clearTimeout(offlineTimeout);
      offlineTimeout = null;
    }
    if (topSpinner) {
      topSpinner.style.display = 'none';
    }
  } else {
    if (!offlineTimeout) {
      offlineTimeout = setTimeout(() => {
        if (topSpinner) {
          topSpinner.style.display = 'flex';
        }
      }, 10000); // 10 seconds without connection
    }
  }
}

// API Fetch Helper: handles both PC Proxy and Mobile Direct Mode
async function nodeApiFetch(endpoint, options = {}) {
  options.headers = options.headers || {};
  let url = '';

  if (isDirectMode) {
    if (!activeServer || !activeServer.url) {
      throw new Error('Укажите URL сервера в настройках');
    }
    url = activeServer.url.replace(/\/$/, '') + endpoint;
    if (activeServer.secret) {
      options.headers['Authorization'] = 'Bearer ' + activeServer.secret;
    }
  } else {
    url = '/api/proxy' + endpoint;
  }

  return fetch(url, options);
}

// ================= SERVERS MANAGEMENT =================

function getLocalServers() {
  try {
    const raw = localStorage.getItem('node_servers');
    if (raw) return JSON.parse(raw);
  } catch (_) {}
  return [
    {
      id: "srv_default",
      name: "VPS Сервер (Узел 1)",
      url: "http://127.0.0.1:9090",
      secret: "",
      is_active: true,
      status: "unknown"
    }
  ];
}

function saveLocalServers(list) {
  localStorage.setItem('node_servers', JSON.stringify(list));
}

async function initServers() {
  let servers = [];
  if (isDirectMode) {
    servers = getLocalServers();
  } else {
    try {
      const res = await fetch('/api/client/servers');
      if (res.ok) {
        servers = await res.json();
      } else {
        servers = getLocalServers();
      }
    } catch (err) {
      servers = getLocalServers();
    }
  }

  renderServerList(servers);
  const active = servers.find(s => s.is_active) || servers[0];
  if (active) {
    setActiveServerState(active);
  }
}

function setActiveServerState(server) {
  activeServer = server;
  document.getElementById('activeServerName').innerText = server.name;
  
  const dot = document.getElementById('activeServerDot');
  dot.className = 'server-status-dot ' + (server.status === 'online' ? 'online' : 'offline');

  refreshHealthBanner();
}

async function refreshHealthBanner() {
  if (!activeServer) {
    setConnectedState(false);
    return;
  }
  const banner = document.getElementById('healthBanner');
  const title = document.getElementById('healthTitle');
  const urlEl = document.getElementById('healthUrl');
  const icon = document.getElementById('healthIcon');

  try {
    const start = Date.now();
    const res = await nodeApiFetch('/api/health');
    if (res.ok) {
      const ping = Date.now() - start;
      const data = await res.json();
      banner.className = 'health-banner online';
      icon.innerHTML = `
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="20 6 9 17 4 12"></polyline>
        </svg>
      `;
      title.innerText = 'Сервер в сети: ' + activeServer.name;
      urlEl.innerText = `${activeServer.url} • ${data.os}/${data.arch} • Ping: ${ping} ms • Uptime: ${data.uptime}`;
      document.getElementById('activeServerDot').className = 'server-status-dot online';

      // Connected!
      setConnectedState(true);
    } else {
      throw new Error('Статус ' + res.status);
    }
  } catch (e) {
    banner.className = 'health-banner offline';
    icon.innerHTML = `
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="10"></circle>
        <line x1="12" y1="8" x2="12" y2="12"></line>
        <line x1="12" y1="16" x2="12.01" y2="16"></line>
      </svg>
    `;
    title.innerText = 'Проверка health... (Недоступен)';
    urlEl.innerText = activeServer.url;
    document.getElementById('activeServerDot').className = 'server-status-dot offline';

    // No connection!
    setConnectedState(false);
  }
}

function checkActiveHealth() {
  refreshHealthBanner();
}

function openServerModal() {
  loadServerModalList();
  openModal('modalServer');
}

async function loadServerModalList() {
  if (isDirectMode) {
    renderServerList(getLocalServers());
    return;
  }
  try {
    const res = await fetch('/api/client/servers');
    const servers = await res.json();
    renderServerList(servers);
  } catch (e) {
    renderServerList(getLocalServers());
  }
}

function renderServerList(servers) {
  const container = document.getElementById('serverListContainer');
  if (!container) return;
  container.innerHTML = '';

  servers.forEach(srv => {
    const div = document.createElement('div');
    div.className = 'server-item-card ' + (srv.is_active ? 'active' : '');
    div.innerHTML = `
      <div class="server-item-info" onclick="selectServer('${srv.id}')">
        <span class="server-status-dot ${srv.status === 'online' ? 'online' : 'offline'}"></span>
        <div class="server-item-text">
          <h4>${escapeHtml(srv.name)} ${srv.is_active ? '<span style="color: var(--cyan-primary); font-size:12px; margin-left:6px;">✓ Активный</span>' : ''}</h4>
          <p>${escapeHtml(srv.url)}${srv.public_host ? ' • Публичный: ' + escapeHtml(srv.public_host) : ''} ${srv.ping_ms ? ' • ' + srv.ping_ms + ' ms' : ''}</p>
        </div>
      </div>
      <div>
        <button class="btn-icon-small delete" title="Удалить сервер" onclick="deleteServer('${srv.id}', event)">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="3 6 5 6 21 6"></polyline>
            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
          </svg>
        </button>
      </div>
    `;
    container.appendChild(div);
  });
}

async function selectServer(id) {
  if (isDirectMode) {
    const list = getLocalServers();
    list.forEach(s => s.is_active = (s.id === id));
    saveLocalServers(list);
    closeModal('modalServer');
    await initServers();
    if (currentView === 'files') loadFiles('');
    return;
  }
  try {
    await fetch('/api/client/servers/active', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id })
    });
    closeModal('modalServer');
    await initServers();
    if (currentView === 'files') loadFiles('');
  } catch (e) {
    alert('Ошибка переключения: ' + e.message);
  }
}

async function addNewServer() {
  const name = document.getElementById('newServerName').value.trim();
  const url = document.getElementById('newServerUrl').value.trim();
  const secret = document.getElementById('newServerSecret').value.trim();
  const publicHostEl = document.getElementById('newServerPublicHost');
  const public_host = publicHostEl ? publicHostEl.value.trim() : '';

  if (!url) {
    alert('Пожалуйста, укажите URL сервера');
    return;
  }

  if (isDirectMode) {
    const list = getLocalServers();
    const newSrv = {
      id: 'srv_' + Date.now(),
      name: name || ('Сервер ' + (list.length + 1)),
      url,
      secret,
      public_host,
      is_active: list.length === 0,
      status: 'unknown'
    };
    list.push(newSrv);
    saveLocalServers(list);
    document.getElementById('newServerName').value = '';
    document.getElementById('newServerUrl').value = '';
    document.getElementById('newServerSecret').value = '';
    if (publicHostEl) publicHostEl.value = '';
    await loadServerModalList();
    await initServers();
    return;
  }

  try {
    const res = await fetch('/api/client/servers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, url, secret, public_host })
    });
    if (res.ok) {
      document.getElementById('newServerName').value = '';
      document.getElementById('newServerUrl').value = '';
      document.getElementById('newServerSecret').value = '';
      if (publicHostEl) publicHostEl.value = '';
      await loadServerModalList();
      await initServers();
    } else {
      const err = await res.json();
      alert('Ошибка: ' + err.error);
    }
  } catch (e) {
    alert('Ошибка добавления: ' + e.message);
  }
}

async function deleteServer(id, e) {
  e.stopPropagation();
  if (!confirm('Удалить этот сервер из списка?')) return;
  if (isDirectMode) {
    let list = getLocalServers();
    list = list.filter(s => s.id !== id);
    if (list.length > 0 && !list.some(s => s.is_active)) list[0].is_active = true;
    saveLocalServers(list);
    await loadServerModalList();
    await initServers();
    return;
  }
  try {
    await fetch(`/api/client/servers?id=${encodeURIComponent(id)}`, { method: 'DELETE' });
    await loadServerModalList();
    await initServers();
  } catch (e) {
    alert('Ошибка удаления: ' + e.message);
  }
}

// ================= FILES MANAGER =================

async function loadFiles(path = '') {
  currentFilesPath = path;
  const listEl = document.getElementById('fileListContainer');
  const emptyEl = document.getElementById('filesEmptyState');
  listEl.innerHTML = '<div style="text-align: center; color: var(--text-muted); padding: 30px;">Загрузка файлов...</div>';
  emptyEl.style.display = 'none';

  try {
    const res = await nodeApiFetch(`/api/files?path=${encodeURIComponent(path)}`);
    if (!res.ok) {
      throw new Error('Ошибка сервера: ' + res.status);
    }
    const data = await res.json();
    renderBreadcrumbs(data.current_path);

    if (!data.items || data.items.length === 0) {
      listEl.innerHTML = '';
      emptyEl.style.display = 'flex';
      return;
    }

    listEl.innerHTML = '';
    data.items.sort((a, b) => {
      if (a.is_dir === b.is_dir) return a.name.localeCompare(b.name);
      return a.is_dir ? -1 : 1;
    });

    data.items.forEach(item => {
      const div = document.createElement('div');
      div.className = 'file-item';
      
      const iconSvg = item.is_dir ? getFolderSvg() : getFileSvg(item.ext);
      const sizeStr = item.is_dir ? 'Папка' : formatFileSize(item.size);
      const dateStr = new Date(item.mod_time).toLocaleString();

      div.innerHTML = `
        <div class="file-info" onclick="${item.is_dir ? `browseFolder('${escapeJs(item.path)}')` : `openFileEditor('${escapeJs(item.path)}')`}">
          <div class="file-icon">${iconSvg}</div>
          <div class="file-details">
            <div class="file-name">${escapeHtml(item.name)}</div>
            <div class="file-meta">${sizeStr} • ${dateStr}</div>
          </div>
        </div>
        <div class="file-actions">
          ${!item.is_dir ? `
            <button class="btn-icon-small" title="Скачать" onclick="downloadFile('${escapeJs(item.path)}')">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
                <polyline points="7 10 12 15 17 10"></polyline>
                <line x1="12" y1="15" x2="12" y2="3"></line>
              </svg>
            </button>
            <button class="btn-icon-small" title="Поделиться публичной ссылкой" onclick="shareFile('${escapeJs(item.path)}')">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path>
                <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path>
              </svg>
            </button>
            <button class="btn-icon-small" title="Редактировать файл" onclick="openFileEditor('${escapeJs(item.path)}')">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path>
                <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
              </svg>
            </button>
          ` : ''}
          <button class="btn-icon-small delete" title="Удалить" onclick="deleteFileItem('${escapeJs(item.path)}')">
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="3 6 5 6 21 6"></polyline>
              <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
            </svg>
          </button>
        </div>
      `;
      listEl.appendChild(div);
    });

  } catch (err) {
    listEl.innerHTML = `<div style="color: var(--danger-text); padding: 30px; text-align: center;">Не удалось загрузить файлы: ${err.message}</div>`;
  }
}

function browseFolder(path) {
  loadFiles(path);
}

function renderBreadcrumbs(currentPath) {
  const container = document.getElementById('filesBreadcrumbs');
  container.innerHTML = '';

  const rootSpan = document.createElement('span');
  rootSpan.className = 'breadcrumb-item';
  rootSpan.innerHTML = `
    <span style="display: inline-flex; align-items: center; gap: 5px;">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"></path>
        <polyline points="9 22 9 12 15 12 15 22"></polyline>
      </svg>
      Корень
    </span>
  `;
  rootSpan.onclick = () => browseFolder('');
  container.appendChild(rootSpan);

  if (!currentPath) return;

  const parts = currentPath.split(/[\\/]/).filter(p => p.length > 0);
  let built = '';
  parts.forEach((p, idx) => {
    const sep = document.createElement('span');
    sep.innerText = ' / ';
    sep.style.opacity = '0.5';
    container.appendChild(sep);

    built += (idx === 0 && currentPath.startsWith('/') ? '/' : '') + p;
    const itemSpan = document.createElement('span');
    itemSpan.className = 'breadcrumb-item';
    itemSpan.innerText = p;
    const targetPath = built;
    itemSpan.onclick = () => browseFolder(targetPath);
    container.appendChild(itemSpan);
  });
}

function openNewFolderModal() {
  document.getElementById('newFolderNameInput').value = '';
  openModal('modalNewFolder');
}

async function createFolder() {
  const name = document.getElementById('newFolderNameInput').value.trim();
  if (!name) return;
  const fullPath = currentFilesPath ? `${currentFilesPath}/${name}` : name;

  try {
    const res = await nodeApiFetch('/api/files/mkdir', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: fullPath })
    });
    if (res.ok) {
      closeModal('modalNewFolder');
      loadFiles(currentFilesPath);
    } else {
      const err = await res.json();
      alert('Ошибка: ' + err.error);
    }
  } catch (e) {
    alert('Ошибка создания папки: ' + e.message);
  }
}

function formatBytes(bytes, decimals = 1) {
  if (!bytes || bytes <= 0) return '0 B';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

function uploadSelectedFile() {
  const input = document.getElementById('fileUploadInput');
  if (!input.files || input.files.length === 0) return;
  const file = input.files[0];

  const progressCard = document.getElementById('fileUploadProgressContainer');
  const nameEl = document.getElementById('uploadProgressFileName');
  const statusEl = document.getElementById('uploadProgressStatus');
  const bytesEl = document.getElementById('uploadProgressBytes');
  const percentEl = document.getElementById('uploadProgressPercent');
  const fillEl = document.getElementById('uploadProgressBarFill');

  // Display initial upload card
  nameEl.innerText = file.name;
  statusEl.innerText = 'Загрузка...';
  bytesEl.innerText = `0 B / ${formatBytes(file.size)}`;
  percentEl.innerText = '0%';
  fillEl.style.width = '0%';
  fillEl.style.background = 'linear-gradient(90deg, #00c2ff, #00e5ff)';
  progressCard.style.display = 'block';

  let targetUrl = '';
  let authHeader = '';
  if (isDirectMode) {
    if (!activeServer || !activeServer.url) {
      alert('Укажите URL сервера в настройках');
      progressCard.style.display = 'none';
      return;
    }
    targetUrl = activeServer.url.replace(/\/$/, '') + '/api/files/upload';
    if (activeServer.secret) {
      authHeader = 'Bearer ' + activeServer.secret;
    }
  } else {
    targetUrl = '/api/proxy/api/files/upload';
  }

  const formData = new FormData();
  formData.append('file', file);
  formData.append('dir', currentFilesPath);

  const xhr = new XMLHttpRequest();
  xhr.open('POST', targetUrl, true);
  if (authHeader) {
    xhr.setRequestHeader('Authorization', authHeader);
  }

  xhr.upload.onprogress = (e) => {
    if (e.lengthComputable) {
      const percent = Math.round((e.loaded / e.total) * 100);
      fillEl.style.width = percent + '%';
      percentEl.innerText = percent + '%';
      bytesEl.innerText = `${formatBytes(e.loaded)} / ${formatBytes(e.total)}`;
      if (percent >= 100) {
        statusEl.innerText = 'Обработка на сервере...';
      }
    }
  };

  xhr.onload = () => {
    if (xhr.status >= 200 && xhr.status < 300) {
      fillEl.style.width = '100%';
      percentEl.innerText = '100%';
      statusEl.innerText = '✓ Файл успешно загружен!';
      fillEl.style.background = 'linear-gradient(90deg, #10b981, #34d399)';
      input.value = '';
      loadFiles(currentFilesPath);
      setTimeout(() => {
        progressCard.style.display = 'none';
      }, 2000);
    } else {
      let errMsg = 'Ошибка загрузки (' + xhr.status + ')';
      try {
        const errJson = JSON.parse(xhr.responseText);
        if (errJson && errJson.error) errMsg = errJson.error;
      } catch (e) {}
      statusEl.innerText = '✕ ' + errMsg;
      fillEl.style.background = 'linear-gradient(90deg, #ef4444, #f87171)';
      alert('Ошибка загрузки файла: ' + errMsg);
      setTimeout(() => {
        progressCard.style.display = 'none';
      }, 3500);
    }
  };

  xhr.onerror = () => {
    statusEl.innerText = '✕ Ошибка сети при передаче';
    fillEl.style.background = 'linear-gradient(90deg, #ef4444, #f87171)';
    alert('Ошибка соединения при передаче файла на сервер');
    setTimeout(() => {
      progressCard.style.display = 'none';
    }, 3500);
  };

  xhr.send(formData);
}

function downloadFile(filePath) {
  if (isDirectMode && activeServer) {
    window.open(`${activeServer.url.replace(/\/$/, '')}/api/files/download?path=${encodeURIComponent(filePath)}&secret=${encodeURIComponent(activeServer.secret || '')}`, '_blank');
  } else {
    window.open(`/api/proxy/api/files/download?path=${encodeURIComponent(filePath)}`, '_blank');
  }
}

async function shareFile(filePath) {
  try {
    const res = await nodeApiFetch('/api/files/share', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: filePath })
    });
    if (!res.ok) {
      const err = await res.json();
      alert('Ошибка: ' + (err.error || 'Не удалось создать ссылку'));
      return;
    }
    const data = await res.json();

    // 1. Check if backend returned full share_url (uses public IP and share port 9091, NOT Tailscale)
    let directShareUrl = data.share_url;

    // 2. If client configured custom public_host for activeServer, use that
    if (activeServer && activeServer.public_host) {
      const sharePort = data.share_port || activeServer.share_port || '9091';
      directShareUrl = `http://${activeServer.public_host}:${sharePort}/share/${data.token}`;
    }

    // 3. Fallback if share_url is empty
    if (!directShareUrl) {
      const sharePort = data.share_port || (activeServer && activeServer.share_port) || '9091';
      let publicHost = data.public_host || (activeServer && activeServer.public_host);
      if (!publicHost && activeServer && activeServer.url) {
        try {
          const u = new URL(activeServer.url);
          publicHost = u.hostname;
        } catch (e) {
          publicHost = window.location.hostname;
        }
      }
      directShareUrl = `http://${publicHost || window.location.hostname}:${sharePort}/share/${data.token}`;
    }

    document.getElementById('shareUrlInput').value = directShareUrl;
    openModal('modalShareFile');
  } catch (e) {
    alert('Ошибка получения ссылки: ' + e.message);
  }
}

function copyShareUrl() {
  const input = document.getElementById('shareUrlInput');
  input.select();
  navigator.clipboard.writeText(input.value);
  alert('Ссылка скопирована в буфер обмена!');
}

async function openFileEditor(filePath) {
  currentEditingPath = filePath;
  document.getElementById('editFileTitle').innerText = 'Файл: ' + filePath.split(/[\\/]/).pop();
  document.getElementById('editFileContent').value = 'Загрузка содержимого...';
  openModal('modalEditFile');

  try {
    const res = await nodeApiFetch(`/api/files/view?path=${encodeURIComponent(filePath)}`);
    if (res.ok) {
      const data = await res.json();
      document.getElementById('editFileContent').value = data.content;
    } else {
      document.getElementById('editFileContent').value = 'Невозможно просмотреть бинарный или защищенный файл.';
    }
  } catch (e) {
    document.getElementById('editFileContent').value = 'Ошибка: ' + e.message;
  }
}

async function saveFileContent() {
  const content = document.getElementById('editFileContent').value;
  try {
    const res = await nodeApiFetch('/api/files/save', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: currentEditingPath, content })
    });
    if (res.ok) {
      alert('Файл сохранен!');
      closeModal('modalEditFile');
      loadFiles(currentFilesPath);
    } else {
      alert('Ошибка сохранения');
    }
  } catch (e) {
    alert('Ошибка: ' + e.message);
  }
}

async function deleteFileItem(filePath) {
  if (!confirm(`Вы действительно хотите удалить "${filePath.split(/[\\/]/).pop()}"?`)) return;
  try {
    const res = await nodeApiFetch('/api/files/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: filePath })
    });
    if (res.ok) {
      loadFiles(currentFilesPath);
    } else {
      alert('Ошибка удаления');
    }
  } catch (e) {
    alert('Ошибка: ' + e.message);
  }
}

// ================= DATABASES MANAGER =================

// ================= DATABASES MANAGER =================
let currentDbTables = [];
let currentActiveTable = '';
let currentTableColumns = [];
let currentTableRows = [];
let currentEditRowIndex = -1;

function switchDbDriver(driver) {
  currentDbDriver = driver;
  document.getElementById('tabPg').className = 'tab-btn ' + (driver === 'postgres' ? 'active' : '');
  document.getElementById('tabMysql').className = 'tab-btn ' + (driver === 'mysql' ? 'active' : '');

  if (driver === 'postgres') {
    document.getElementById('dbPortInput').value = '5432';
    document.getElementById('dbUserInput').value = 'postgres';
    document.getElementById('dbNameInput').value = 'postgres';
  } else {
    document.getElementById('dbPortInput').value = '3306';
    document.getElementById('dbUserInput').value = 'root';
    document.getElementById('dbNameInput').value = 'mysql';
  }
  loadDatabasesList();
}

function switchDbSubTab(tab) {
  currentDbSubTab = tab;
  document.getElementById('subTabTables').className = 'tab-btn ' + (tab === 'tables' ? 'active' : '');
  document.getElementById('subTabSQL').className = 'tab-btn ' + (tab === 'sql' ? 'active' : '');
  document.getElementById('subTabKV').className = 'tab-btn ' + (tab === 'kv' ? 'active' : '');

  document.getElementById('panelDbTables').style.display = tab === 'tables' ? 'block' : 'none';
  document.getElementById('panelDbSQL').style.display = tab === 'sql' ? 'block' : 'none';
  document.getElementById('panelDbKV').style.display = tab === 'kv' ? 'block' : 'none';
}

function getDbRequestPayload() {
  const dropdown = document.getElementById('dbSelectDropdown');
  const selectedDb = dropdown && dropdown.value ? dropdown.value : (document.getElementById('dbNameInput').value.trim() || (currentDbDriver === 'postgres' ? 'postgres' : 'mysql'));

  return {
    driver: currentDbDriver,
    host: document.getElementById('dbHostInput').value.trim() || 'localhost',
    port: parseInt(document.getElementById('dbPortInput').value) || (currentDbDriver === 'postgres' ? 5432 : 3306),
    user: document.getElementById('dbUserInput').value.trim(),
    password: document.getElementById('dbPassInput').value,
    db_name: selectedDb,
    path: document.getElementById('dbPathInput').value.trim(),
    value: document.getElementById('dbValueInput').value,
    merge: document.getElementById('dbMergeCheck').checked,
    force: document.getElementById('dbForceCheck').checked
  };
}

function toggleDbConfigModal() {
  openModal('modalDbConfig');
}

async function testDbConnection(silent = false) {
  const payload = getDbRequestPayload();
  try {
    const res = await nodeApiFetch('/api/db/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if (res.ok) {
      if (!silent) alert('Успешно: ' + (data.message || 'Соединение установлено!'));
      closeModal('modalDbConfig');
      loadDatabasesList();
    } else {
      if (!silent) alert('Ошибка соединения: ' + data.error);
    }
  } catch (e) {
    if (!silent) alert('Ошибка: ' + e.message);
  }
}

async function loadDatabasesList() {
  const payload = getDbRequestPayload();
  const dropdown = document.getElementById('dbSelectDropdown');
  if (!dropdown) return;

  try {
    const res = await nodeApiFetch('/api/db/list', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    if (res.ok) {
      const dbs = await res.json();
      dropdown.innerHTML = '';
      if (Array.isArray(dbs) && dbs.length > 0) {
        dbs.forEach(name => {
          const opt = document.createElement('option');
          opt.value = name;
          opt.innerText = name;
          if (name === payload.db_name) opt.selected = true;
          dropdown.appendChild(opt);
        });
      } else {
        const opt = document.createElement('option');
        opt.value = payload.db_name;
        opt.innerText = payload.db_name;
        dropdown.appendChild(opt);
      }
    }
  } catch (e) {
    console.error('Ошибка загрузки списка БД:', e);
  }

  loadTablesList();
}

function onDatabaseChange() {
  const dropdown = document.getElementById('dbSelectDropdown');
  if (dropdown && dropdown.value) {
    document.getElementById('dbNameInput').value = dropdown.value;
  }
  loadTablesList();
}

async function dropCurrentDatabase() {
  const dropdown = document.getElementById('dbSelectDropdown');
  const dbName = dropdown ? dropdown.value : '';
  if (!dbName || dbName === 'postgres' || dbName === 'mysql') {
    alert('Нельзя удалить системную базу данных!');
    return;
  }
  if (!confirm(`ВНИМАНИЕ! Вы точно хотите безвозвратно удалить базу данных "${dbName}"?`)) return;

  const payload = getDbRequestPayload();
  payload.db_name = dbName;

  try {
    const res = await nodeApiFetch('/api/db/drop', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if (res.ok) {
      alert(data.message || 'База удалена!');
      loadDatabasesList();
    } else {
      alert('Ошибка: ' + data.error);
    }
  } catch (e) {
    alert('Ошибка: ' + e.message);
  }
}

async function loadTablesList() {
  const container = document.getElementById('dbTablesContainer');
  if (!container) return;
  container.innerHTML = '<div style="padding: 12px; color: var(--text-dim); font-size: 12px;">Загрузка таблиц...</div>';

  const payload = getDbRequestPayload();
  try {
    const res = await nodeApiFetch('/api/db/tables', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    if (!res.ok) {
      container.innerHTML = '<div style="padding: 12px; color: var(--danger-text); font-size: 12px;">Ошибка подключения к БД</div>';
      return;
    }
    const tables = await res.json();
    currentDbTables = Array.isArray(tables) ? tables : [];

    if (currentDbTables.length === 0) {
      container.innerHTML = '<div style="padding: 12px; color: var(--text-dim); font-size: 12px;">В базе нет таблиц</div>';
      document.getElementById('activeTableName').innerText = 'Нет таблиц';
      document.getElementById('activeTableStats').innerText = 'Создайте таблицы через SQL консоль';
      document.getElementById('dbGridContainer').innerHTML = '<div style="padding: 30px; text-align: center; color: var(--text-muted);">В выбранной базе данных нет созданных таблиц.</div>';
      document.getElementById('btnInsertRow').style.display = 'none';
      document.getElementById('btnRefreshTable').style.display = 'none';
      return;
    }

    let html = '';
    currentDbTables.forEach(t => {
      const isActive = t === currentActiveTable;
      html += `
        <div class="db-table-item ${isActive ? 'active' : ''}" onclick="selectDbTable('${escapeJs(t)}')">
          <span style="display: flex; align-items: center; gap: 8px;">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="3" width="18" height="18" rx="2"></rect>
              <line x1="3" y1="9" x2="21" y2="9"></line>
              <line x1="9" y1="21" x2="9" y2="9"></line>
            </svg>
            ${escapeHtml(t)}
          </span>
          <span style="font-size: 11px; opacity: 0.6;">таблица</span>
        </div>
      `;
    });
    container.innerHTML = html;

    // If active table is in list, keep it, else select first
    if (!currentDbTables.includes(currentActiveTable)) {
      selectDbTable(currentDbTables[0]);
    } else {
      selectDbTable(currentActiveTable);
    }
  } catch (e) {
    container.innerHTML = `<div style="padding: 12px; color: var(--danger-text); font-size: 12px;">Ошибка: ${e.message}</div>`;
  }
}

function selectDbTable(tableName) {
  currentActiveTable = tableName;

  // Highlight in sidebar
  const items = document.querySelectorAll('.db-table-item');
  items.forEach(el => {
    if (el.innerText.includes(tableName)) {
      el.classList.add('active');
    } else {
      el.classList.remove('active');
    }
  });

  document.getElementById('activeTableName').innerHTML = `
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--cyan-primary)" stroke-width="2">
      <rect x="3" y="3" width="18" height="18" rx="2"></rect>
      <line x1="3" y1="9" x2="21" y2="9"></line>
      <line x1="9" y1="21" x2="9" y2="9"></line>
    </svg>
    ${escapeHtml(tableName)}
  `;
  document.getElementById('btnInsertRow').style.display = 'inline-flex';
  document.getElementById('btnRefreshTable').style.display = 'inline-flex';

  loadTableData(tableName);
}

function refreshCurrentTable() {
  if (currentActiveTable) {
    loadTableData(currentActiveTable);
  }
}

async function loadTableData(tableName) {
  const grid = document.getElementById('dbGridContainer');
  grid.innerHTML = '<div style="padding: 30px; text-align: center; color: var(--text-muted);">Загрузка данных таблицы...</div>';

  const payload = getDbRequestPayload();
  const body = {
    ...payload,
    table: tableName,
    limit: 100,
    offset: 0
  };

  try {
    const res = await nodeApiFetch('/api/db/table/data', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    const data = await res.json();
    if (!res.ok) {
      grid.innerHTML = `<div style="padding: 24px; color: var(--danger-text);">❌ Ошибка: ${escapeHtml(data.error)}</div>`;
      return;
    }

    currentTableColumns = data.columns || [];
    currentTableRows = data.rows || [];

    document.getElementById('activeTableStats').innerText = `Показано записей: ${currentTableRows.length} (${data.execution_ms} ms)`;

    if (currentTableColumns.length === 0) {
      grid.innerHTML = '<div style="padding: 30px; text-align: center; color: var(--text-muted);">Таблица не содержит колонок.</div>';
      return;
    }

    let html = '<table class="db-data-grid"><thead><tr>';
    html += '<th style="width: 90px; text-align: center;">Действия</th>';
    currentTableColumns.forEach(col => {
      html += `<th>${escapeHtml(col)}</th>`;
    });
    html += '</tr></thead><tbody>';

    if (currentTableRows.length === 0) {
      html += `<tr><td colspan="${currentTableColumns.length + 1}" style="text-align: center; padding: 30px; color: var(--text-dim);">Таблица пуста. Нажмите "+ Добавить запись" выше.</td></tr>`;
    } else {
      currentTableRows.forEach((row, rIdx) => {
        html += '<tr>';
        html += `
          <td style="text-align: center;">
            <div class="db-row-actions">
              <button class="db-action-pill edit" onclick="openEditRowModal(${rIdx})" title="Изменить строку">Изменить</button>
              <button class="db-action-pill delete" onclick="deleteTableRow(${rIdx})" title="Удалить строку">✕</button>
            </div>
          </td>
        `;
        row.forEach(cell => {
          const val = cell === null ? '<span style="opacity:0.35; font-style:italic;">NULL</span>' : escapeHtml(String(cell));
          html += `<td>${val}</td>`;
        });
        html += '</tr>';
      });
    }

    html += '</tbody></table>';
    grid.innerHTML = html;
  } catch (e) {
    grid.innerHTML = `<div style="padding: 24px; color: var(--danger-text);">❌ Ошибка: ${e.message}</div>`;
  }
}

// Modal: Insert Row
function openInsertRowModal() {
  if (!currentActiveTable || currentTableColumns.length === 0) return;
  document.getElementById('modalInsertRowTitle').innerText = `Добавить запись в "${currentActiveTable}"`;

  const container = document.getElementById('modalInsertRowFields');
  let html = '';
  currentTableColumns.forEach(col => {
    html += `
      <div class="form-group" style="margin-bottom: 12px;">
        <label class="form-label" style="font-size: 13px;">${escapeHtml(col)}</label>
        <input type="text" id="insert_col_${escapeHtml(col)}" class="form-control" placeholder="Значение для ${escapeHtml(col)}">
      </div>
    `;
  });
  container.innerHTML = html;
  openModal('modalInsertRow');
}

async function submitInsertRow() {
  const values = {};
  currentTableColumns.forEach(col => {
    const el = document.getElementById(`insert_col_${col}`);
    if (el && el.value.trim() !== '') {
      values[col] = el.value.trim();
    }
  });

  const payload = getDbRequestPayload();
  const body = {
    ...payload,
    table: currentActiveTable,
    values: values
  };

  try {
    const res = await nodeApiFetch('/api/db/table/insert', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    const data = await res.json();
    if (res.ok) {
      closeModal('modalInsertRow');
      loadTableData(currentActiveTable);
    } else {
      alert('Ошибка добавления: ' + data.error);
    }
  } catch (e) {
    alert('Ошибка: ' + e.message);
  }
}

// Modal: Edit Row
function openEditRowModal(rowIndex) {
  if (rowIndex < 0 || rowIndex >= currentTableRows.length) return;
  currentEditRowIndex = rowIndex;
  const row = currentTableRows[rowIndex];

  document.getElementById('modalEditRowTitle').innerText = `Редактирование записи в "${currentActiveTable}"`;
  const container = document.getElementById('modalEditRowFields');

  let html = '';
  currentTableColumns.forEach((col, cIdx) => {
    const val = row[cIdx] !== null && row[cIdx] !== undefined ? String(row[cIdx]) : '';
    const isPk = cIdx === 0; // First column as primary key
    html += `
      <div class="form-group" style="margin-bottom: 12px;">
        <label class="form-label" style="font-size: 13px;">
          ${escapeHtml(col)} ${isPk ? '<span style="color:var(--cyan-primary); font-size:11px;">(Идентификатор)</span>' : ''}
        </label>
        <input type="text" id="edit_col_${escapeHtml(col)}" class="form-control" value="${escapeHtml(val)}" ${isPk ? 'readonly style="opacity:0.75;"' : ''}>
      </div>
    `;
  });
  container.innerHTML = html;
  openModal('modalEditRow');
}

async function submitEditRow() {
  if (currentEditRowIndex < 0 || currentEditRowIndex >= currentTableRows.length) return;
  const row = currentTableRows[currentEditRowIndex];
  const pkCol = currentTableColumns[0];
  const pkVal = row[0];

  const values = {};
  currentTableColumns.forEach((col, cIdx) => {
    if (cIdx === 0) return; // Do not update PK itself
    const el = document.getElementById(`edit_col_${col}`);
    if (el) {
      values[col] = el.value;
    }
  });

  const payload = getDbRequestPayload();
  const body = {
    ...payload,
    table: currentActiveTable,
    pk_col: pkCol,
    pk_val: pkVal,
    values: values
  };

  try {
    const res = await nodeApiFetch('/api/db/table/update', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    const data = await res.json();
    if (res.ok) {
      closeModal('modalEditRow');
      loadTableData(currentActiveTable);
    } else {
      alert('Ошибка обновления: ' + data.error);
    }
  } catch (e) {
    alert('Ошибка: ' + e.message);
  }
}

async function deleteTableRow(rowIndex) {
  if (rowIndex < 0 || rowIndex >= currentTableRows.length) return;
  const row = currentTableRows[rowIndex];
  const pkCol = currentTableColumns[0];
  const pkVal = row[0];

  if (!confirm(`Вы действительно хотите удалить эту запись (${pkCol} = ${pkVal})?`)) return;

  const payload = getDbRequestPayload();
  const body = {
    ...payload,
    table: currentActiveTable,
    pk_col: pkCol,
    pk_val: pkVal
  };

  try {
    const res = await nodeApiFetch('/api/db/table/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    const data = await res.json();
    if (res.ok) {
      loadTableData(currentActiveTable);
    } else {
      alert('Ошибка удаления: ' + data.error);
    }
  } catch (e) {
    alert('Ошибка: ' + e.message);
  }
}

function openCreateDbModal() {
  document.getElementById('newDbNameInput').value = '';
  openModal('modalCreateDb');
}

async function createDatabaseSubmit() {
  const name = document.getElementById('newDbNameInput').value.trim();
  if (!name) return;
  const payload = getDbRequestPayload();
  payload.db_name = name;

  try {
    const res = await nodeApiFetch('/api/db/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if (res.ok) {
      alert(data.message || 'База успешно создана!');
      closeModal('modalCreateDb');
      loadDatabasesList();
    } else {
      alert('Ошибка: ' + data.error);
    }
  } catch (e) {
    alert('Ошибка: ' + e.message);
  }
}

async function dbReadKV() {
  const payload = getDbRequestPayload();
  try {
    const res = await nodeApiFetch('/api/db/kv/read', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if (res.ok) {
      document.getElementById('dbValueInput').value = data.value;
    } else {
      alert('Ошибка чтения: ' + data.error);
    }
  } catch (e) {
    alert('Ошибка: ' + e.message);
  }
}

async function dbWriteKV() {
  const payload = getDbRequestPayload();
  try {
    const res = await nodeApiFetch('/api/db/kv/write', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if (res.ok) {
      alert(data.message || 'Значение записано!');
    } else {
      alert('Ошибка записи: ' + data.error);
    }
  } catch (e) {
    alert('Ошибка: ' + e.message);
  }
}

async function dbDeleteKV() {
  if (!confirm('Вы уверены, что хотите удалить указанное значение/ключ?')) return;
  const payload = getDbRequestPayload();
  try {
    const res = await nodeApiFetch('/api/db/kv/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if (res.ok) {
      alert(data.message || 'Удалено!');
      document.getElementById('dbValueInput').value = '';
    } else {
      alert('Ошибка: ' + data.error);
    }
  } catch (e) {
    alert('Ошибка: ' + e.message);
  }
}

async function executeSqlQuery() {
  const query = document.getElementById('dbSqlQueryInput').value.trim();
  if (!query) return;

  const payload = getDbRequestPayload();
  payload.query = query;

  const resultContainer = document.getElementById('sqlQueryResult');
  resultContainer.style.display = 'block';
  resultContainer.innerHTML = '<div style="padding: 14px; color: var(--text-muted);">Выполнение запроса...</div>';

  try {
    const res = await nodeApiFetch('/api/db/query', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if (!res.ok) {
      resultContainer.innerHTML = `<div style="padding: 14px; color: var(--danger-text);">❌ ${escapeHtml(data.error)}</div>`;
      return;
    }

    if (data.columns && data.columns.length > 0) {
      let tableHtml = `<div style="padding: 10px 14px; font-size: 12px; color: var(--text-muted);">${data.message} (${data.execution_ms} ms)</div>`;
      tableHtml += '<table class="sql-table"><thead><tr>';
      data.columns.forEach(col => {
        tableHtml += `<th>${escapeHtml(col)}</th>`;
      });
      tableHtml += '</tr></thead><tbody>';

      (data.rows || []).forEach(row => {
        tableHtml += '<tr>';
        row.forEach(cell => {
          tableHtml += `<td>${cell === null ? '<span style="opacity:0.4;">NULL</span>' : escapeHtml(String(cell))}</td>`;
        });
        tableHtml += '</tr>';
      });

      tableHtml += '</tbody></table>';
      resultContainer.innerHTML = tableHtml;
    } else {
      resultContainer.innerHTML = `<div style="padding: 14px; color: var(--success-text);">✓ ${escapeHtml(data.message)} (${data.execution_ms} ms)</div>`;
    }
  } catch (e) {
    resultContainer.innerHTML = `<div style="padding: 14px; color: var(--danger-text);">❌ ${e.message}</div>`;
  }
}

// ================= SCRIPTS MANAGER =================
let scriptIsRunning = false;
let scriptWS = null;
let currentViewingSnapshot = '';

// Update Single Toggle Button
function updateScriptToggleUI(isRunning, lang = currentScriptLang) {
  scriptIsRunning = isRunning;
  const btn = document.getElementById('btnScriptToggle');
  const icon = document.getElementById('btnScriptToggleIcon');
  const text = document.getElementById('btnScriptToggleText');
  const badge = document.getElementById('scriptStatusBadge');

  if (!btn) return;

  const ext = lang === 'python' ? '.py' : (lang === 'nodejs' ? '.js' : '.go');

  if (isRunning) {
    btn.className = 'btn-script-toggle running';
    btn.title = 'Остановить выполнение скрипта';
    icon.innerHTML = '<rect x="5" y="5" width="14" height="14" rx="2"></rect>';
    text.innerText = 'Остановить';
    if (badge) {
      badge.className = 'log-status-badge running';
      badge.innerText = 'Выполняется';
    }
  } else {
    btn.className = 'btn-script-toggle idle';
    btn.title = 'Запустить скрипт';
    icon.innerHTML = '<polygon points="5 3 19 12 5 21 5 3"></polygon>';
    text.innerText = `Запустить ${ext}`;
    if (badge) {
      badge.className = 'log-status-badge';
      badge.innerText = 'Остановлен';
    }
  }
}

function toggleScriptExecution() {
  if (scriptIsRunning) {
    stopScript();
  } else {
    runScript();
  }
}

function setScriptLang(lang) {
  currentScriptLang = lang;
  document.getElementById('langPy').className = 'tab-btn ' + (lang === 'python' ? 'active' : '');
  document.getElementById('langNode').className = 'tab-btn ' + (lang === 'nodejs' ? 'active' : '');
  document.getElementById('langGo').className = 'tab-btn ' + (lang === 'golang' ? 'active' : '');

  updateScriptToggleUI(scriptIsRunning, lang);
  loadCurrentScript(lang);
}

function onScriptIdChange() {
  const id = document.getElementById('scriptIdInput').value.trim() || 'main';
  loadScriptSnapshots(id);
}

async function loadCurrentScript(lang = currentScriptLang) {
  const id = document.getElementById('scriptIdInput').value.trim() || 'main';
  try {
    const res = await nodeApiFetch(`/api/scripts/get?id=${encodeURIComponent(id)}&lang=${encodeURIComponent(lang)}`);
    if (res.ok) {
      const data = await res.json();
      if (data.content !== undefined) {
        document.getElementById('scriptContentInput').value = data.content;
      }
      // Keep the user-selected language tab active
      if (lang) {
        currentScriptLang = lang;
        document.getElementById('langPy').className = 'tab-btn ' + (lang === 'python' ? 'active' : '');
        document.getElementById('langNode').className = 'tab-btn ' + (lang === 'nodejs' ? 'active' : '');
        document.getElementById('langGo').className = 'tab-btn ' + (lang === 'golang' ? 'active' : '');
        updateScriptToggleUI(scriptIsRunning, lang);
      } else if (data.language) {
        currentScriptLang = data.language;
        document.getElementById('langPy').className = 'tab-btn ' + (data.language === 'python' ? 'active' : '');
        document.getElementById('langNode').className = 'tab-btn ' + (data.language === 'nodejs' ? 'active' : '');
        document.getElementById('langGo').className = 'tab-btn ' + (data.language === 'golang' ? 'active' : '');
        updateScriptToggleUI(scriptIsRunning, data.language);
      }
    }
  } catch (e) {
    console.error(e);
  }
  loadScriptSnapshots(id);
  connectScriptWS(id);
}

function loadScriptFromFile(event) {
  const file = event.target.files[0];
  if (!file) return;
  handleIncomingScriptFile(file);
}

function handleIncomingScriptFile(file) {
  const baseName = file.name.replace(/\.[^/.]+$/, "");
  document.getElementById('scriptIdInput').value = baseName || 'main';

  if (file.name.endsWith('.py')) setScriptLang('python');
  else if (file.name.endsWith('.js') || file.name.endsWith('.ts')) setScriptLang('nodejs');
  else if (file.name.endsWith('.go')) setScriptLang('golang');

  const reader = new FileReader();
  reader.onload = async function(e) {
    document.getElementById('scriptContentInput').value = e.target.result;
    // Auto-save into folder and snapshots
    await saveCurrentScript();
  };
  reader.readAsText(file);
}

// Drag & Drop Setup
document.addEventListener('DOMContentLoaded', () => {
  const dropZone = document.getElementById('editorDropZone');
  if (dropZone) {
    ['dragenter', 'dragover'].forEach(eventName => {
      dropZone.addEventListener(eventName, (e) => {
        e.preventDefault();
        e.stopPropagation();
        dropZone.classList.add('dragover');
      }, false);
    });

    ['dragleave', 'drop'].forEach(eventName => {
      dropZone.addEventListener(eventName, (e) => {
        e.preventDefault();
        e.stopPropagation();
        dropZone.classList.remove('dragover');
      }, false);
    });

    dropZone.addEventListener('drop', (e) => {
      const dt = e.dataTransfer;
      const files = dt.files;
      if (files.length > 0) {
        handleIncomingScriptFile(files[0]);
      }
    }, false);
  }
});

async function saveCurrentScript() {
  const id = document.getElementById('scriptIdInput').value.trim() || 'main';
  const content = document.getElementById('scriptContentInput').value;

  try {
    const res = await nodeApiFetch('/api/scripts/save', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        id,
        language: currentScriptLang,
        content,
        auto_deps: true
      })
    });
    if (res.ok) {
      loadScriptsList();
      loadScriptSnapshots(id);
    }
  } catch (e) {
    console.error(e);
  }
}

async function deleteCurrentScript() {
  const id = document.getElementById('scriptIdInput').value.trim() || 'main';
  if (!confirm(`Удалить скрипт "${id}" и все его снимки?`)) return;

  try {
    const res = await nodeApiFetch('/api/scripts/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id })
    });
    if (res.ok) {
      document.getElementById('scriptIdInput').value = 'main';
      document.getElementById('scriptContentInput').value = '';
      loadScriptsList();
      loadScriptSnapshots('main');
    }
  } catch (e) {
    alert('Ошибка: ' + e.message);
  }
}

async function loadScriptsList() {
  const container = document.getElementById('savedScriptsChips');
  if (!container) return;

  try {
    const res = await nodeApiFetch('/api/scripts/list');
    if (res.ok) {
      const list = await res.json();
      if (!Array.isArray(list) || list.length === 0) {
        container.innerHTML = '<div style="font-size: 12px; color: var(--text-dim);">Нет сохраненных скриптов</div>';
        return;
      }

      let html = '';
      const currentId = document.getElementById('scriptIdInput').value.trim();
      list.forEach(item => {
        const isActive = item.id === currentId;
        const statusDot = item.status === 'running' ? '🟢' : '⚪';
        html += `
          <div class="script-chip ${isActive ? 'active' : ''}" onclick="selectSavedScript('${escapeJs(item.id)}', '${escapeJs(item.language)}')">
            <span>${statusDot} ${escapeHtml(item.id)}</span>
            <span class="script-chip-badge ${escapeHtml(item.language)}">${escapeHtml(item.language)}</span>
          </div>
        `;
      });
      container.innerHTML = html;
    }
  } catch (e) {
    console.error(e);
  }
}

function selectSavedScript(id, lang) {
  document.getElementById('scriptIdInput').value = id;
  setScriptLang(lang);
  loadCurrentScript();
}

async function loadScriptSnapshots(id) {
  const container = document.getElementById('scriptSnapshotsChips');
  if (!container) return;

  try {
    const res = await nodeApiFetch(`/api/scripts/history?id=${encodeURIComponent(id)}`);
    if (res.ok) {
      const items = await res.json();
      if (!Array.isArray(items) || items.length === 0) {
        container.innerHTML = '<div style="font-size: 12px; color: var(--text-dim);">История версий пуста</div>';
        return;
      }

      let html = '';
      items.forEach(item => {
        const dateStr = new Date(item.timestamp).toLocaleString('ru-RU', {
          month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit'
        });
        html += `
          <div class="script-chip" onclick="openSnapshot('${escapeJs(id)}', '${escapeJs(item.file_name)}')" title="Просмотреть снимок">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"></circle>
              <polyline points="12 6 12 12 14 14"></polyline>
            </svg>
            <span>${escapeHtml(dateStr)}</span>
            <span style="font-size: 10px; opacity: 0.6;">${formatFileSize(item.size)}</span>
          </div>
        `;
      });
      container.innerHTML = html;
    }
  } catch (e) {
    console.error(e);
  }
}

async function openSnapshot(id, fileName) {
  currentViewingSnapshot = fileName;
  try {
    const res = await nodeApiFetch(`/api/scripts/history/content?id=${encodeURIComponent(id)}&file=${encodeURIComponent(fileName)}`);
    if (res.ok) {
      const data = await res.json();
      document.getElementById('modalSnapshotTitle').innerText = `Снимок: ${fileName}`;
      document.getElementById('modalSnapshotContent').value = data.content;
      openModal('modalViewSnapshot');
    }
  } catch (e) {
    alert('Ошибка загрузки снимка: ' + e.message);
  }
}

function restoreCurrentSnapshot() {
  const content = document.getElementById('modalSnapshotContent').value;
  document.getElementById('scriptContentInput').value = content;
  closeModal('modalViewSnapshot');
  saveCurrentScript();
  alert('Версия восстановлена!');
}

async function runScript() {
  const id = document.getElementById('scriptIdInput').value.trim() || 'main';
  const content = document.getElementById('scriptContentInput').value;
  const autoDeps = document.getElementById('scriptAutoDeps').checked;

  const terminal = document.getElementById('scriptLogsTerminal');
  terminal.innerText = `[Node] 🚀 Запуск скрипта '${id}'...\n`;
  updateScriptToggleUI(true, currentScriptLang);

  try {
    const res = await nodeApiFetch('/api/scripts/run', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        id,
        language: currentScriptLang,
        content,
        auto_deps: autoDeps
      })
    });

    if (!res.ok) {
      const err = await res.json();
      terminal.innerText += `❌ Ошибка: ${err.error}\n`;
      updateScriptToggleUI(false, currentScriptLang);
      return;
    }

    connectScriptWS(id);
    loadScriptsList();
    loadScriptSnapshots(id);
  } catch (e) {
    terminal.innerText += `❌ Ошибка запроса: ${e.message}\n`;
    updateScriptToggleUI(false, currentScriptLang);
  }
}

async function stopScript() {
  const id = document.getElementById('scriptIdInput').value.trim() || 'main';
  const terminal = document.getElementById('scriptLogsTerminal');
  terminal.innerText += `\n[Node] ⚠️ Остановка процесса...\n`;

  try {
    await nodeApiFetch('/api/scripts/stop', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id })
    });
    updateScriptToggleUI(false, currentScriptLang);
    loadScriptsList();
  } catch (e) {
    console.error(e);
  }
}

function clearTerminalLogs() {
  document.getElementById('scriptLogsTerminal').innerText = '';
}

// WebSocket Real-time Log Streaming
function connectScriptWS(id) {
  if (scriptWS) {
    try { scriptWS.close(); } catch (_) {}
    scriptWS = null;
  }

  const terminal = document.getElementById('scriptLogsTerminal');
  const badge = document.getElementById('scriptStatusBadge');

  let wsUrl = '';
  if (isDirectMode && activeServer) {
    const base = activeServer.url.replace(/^http/, 'ws').replace(/\/$/, '');
    wsUrl = `${base}/api/scripts/ws?id=${encodeURIComponent(id)}&secret=${encodeURIComponent(activeServer.secret || '')}`;
  } else {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    wsUrl = `${protocol}//${window.location.host}/api/proxy/api/scripts/ws?id=${encodeURIComponent(id)}`;
  }

  try {
    scriptWS = new WebSocket(wsUrl);

    scriptWS.onopen = () => {
      // WebSocket connected
    };

    scriptWS.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.type === 'init') {
          terminal.innerText = data.logs || 'Логи пусты.\n';
          terminal.scrollTop = terminal.scrollHeight;
          const isRun = data.status === 'running' || data.status === 'installing_deps' || data.status === 'starting';
          updateScriptToggleUI(isRun, currentScriptLang);
        } else if (data.type === 'log') {
          if (data.log) {
            terminal.innerText += data.log;
            terminal.scrollTop = terminal.scrollHeight;
          }
          if (data.status) {
            const isRun = data.status === 'running' || data.status === 'installing_deps' || data.status === 'starting';
            updateScriptToggleUI(isRun, currentScriptLang);
          }
        } else if (data.type === 'status') {
          if (data.status) {
            const isRun = data.status === 'running' || data.status === 'installing_deps' || data.status === 'starting';
            updateScriptToggleUI(isRun, currentScriptLang);
          }
        }
      } catch (err) {
        terminal.innerText += event.data;
        terminal.scrollTop = terminal.scrollHeight;
      }
    };

    scriptWS.onerror = (e) => {
      // Fallback: poll or do nothing
    };

    scriptWS.onclose = () => {
      // Closed cleanly
    };
  } catch (err) {
    console.error('WebSocket connection error:', err);
  }
}

// ================= MODAL HELPERS =================

function openModal(id) {
  const el = document.getElementById(id);
  if (el) el.style.display = 'flex';
}

function closeModal(id) {
  const el = document.getElementById(id);
  if (el) el.style.display = 'none';
}

function closeModalOnBackdrop(e, id) {
  if (e.target.id === id) {
    closeModal(id);
  }
}

// Utility Helpers
function formatFileSize(bytes) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

// Clean Vector SVG Icons
function getFolderSvg() {
  return `
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#00c2ff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
    </svg>
  `;
}

function getFileSvg(ext) {
  ext = (ext || '').toLowerCase();
  let color = '#94a3b8';
  if (ext === '.py') color = '#38bdf8';
  else if (ext === '.js' || ext === '.ts') color = '#fbbf24';
  else if (ext === '.go') color = '#00c2ff';
  else if (ext === '.json' || ext === '.yaml' || ext === '.yml') color = '#a78bfa';
  else if (ext === '.zip' || ext === '.tar' || ext === '.gz') color = '#f43f5e';

  return `
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="${color}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"></path>
      <polyline points="13 2 13 9 20 9"></polyline>
    </svg>
  `;
}

function escapeHtml(str) {
  if (!str) return '';
  return str.replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');
}

function escapeJs(str) {
  if (!str) return '';
  return str.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
}
