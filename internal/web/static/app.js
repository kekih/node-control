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

// Initialize
document.addEventListener('DOMContentLoaded', () => {
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

  const backBtn = document.getElementById('btnBack');
  const title = document.getElementById('headerTitle');
  const subtitle = document.getElementById('headerSubtitle');

  title.innerText = 'Node';

  if (view === 'dashboard') {
    backBtn.style.display = 'none';
    subtitle.innerText = 'Клиент сервера';
    refreshHealthBanner();
  } else if (view === 'files') {
    backBtn.style.display = 'flex';
    subtitle.innerText = 'Файлы';
    loadFiles(currentFilesPath);
  } else if (view === 'db') {
    backBtn.style.display = 'flex';
    subtitle.innerText = 'Базы данных';
  } else if (view === 'scripts') {
    backBtn.style.display = 'flex';
    subtitle.innerText = 'Скрипты';
    loadCurrentScript();
  }
}

function refreshCurrentView() {
  if (currentView === 'dashboard') {
    refreshHealthBanner();
  } else if (currentView === 'files') {
    loadFiles(currentFilesPath);
  } else if (currentView === 'scripts') {
    fetchScriptLogs();
  } else if (currentView === 'db') {
    testDbConnection(true);
  }
}

// ================= TOP SPINNER & HEALTH LOGIC =================
// "когда подключился к серверу этот круг пропадает, а когда подключения нету 10 секунд то появляется"

function setConnectedState(isConnected) {
  const topSpinner = document.getElementById('topSpinner');

  if (isConnected) {
    // Connection established! Cancel offline timer and hide circle immediately
    if (offlineTimeout) {
      clearTimeout(offlineTimeout);
      offlineTimeout = null;
    }
    if (topSpinner) {
      topSpinner.style.display = 'none';
    }
  } else {
    // No connection! Start 10-second timer if not already ticking
    if (!offlineTimeout) {
      offlineTimeout = setTimeout(() => {
        if (topSpinner) {
          topSpinner.style.display = 'flex';
        }
      }, 10000); // 10 seconds without connection
    }
  }
}

// ================= SERVERS MANAGEMENT =================

async function initServers() {
  try {
    const res = await fetch('/api/client/servers');
    const servers = await res.json();
    renderServerList(servers);

    const active = servers.find(s => s.is_active) || servers[0];
    if (active) {
      setActiveServerState(active);
    }
  } catch (err) {
    console.error('Ошибка загрузки списка серверов:', err);
    setConnectedState(false);
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
    const res = await fetch('/api/proxy/api/health');
    if (res.ok) {
      const data = await res.json();
      banner.className = 'health-banner online';
      icon.innerHTML = `
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="20 6 9 17 4 12"></polyline>
        </svg>
      `;
      title.innerText = 'Сервер в сети: ' + activeServer.name;
      urlEl.innerText = `${activeServer.url} • ${data.os}/${data.arch} • Uptime: ${data.uptime}`;
      document.getElementById('activeServerDot').className = 'server-status-dot online';

      // Connected!
      setConnectedState(true);
    } else {
      throw new Error('Статус ' + res.status);
    }
  } catch (e) {
    banner.className = 'health-banner offline';
    icon.innerHTML = `
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
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
  try {
    const res = await fetch('/api/client/servers');
    const servers = await res.json();
    renderServerList(servers);
  } catch (e) {
    console.error(e);
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
          <p>${escapeHtml(srv.url)} ${srv.ping_ms ? ' • ' + srv.ping_ms + ' ms' : ''}</p>
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

  if (!url) {
    alert('Пожалуйста, укажите URL сервера');
    return;
  }

  try {
    const res = await fetch('/api/client/servers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, url, secret })
    });
    if (res.ok) {
      document.getElementById('newServerName').value = '';
      document.getElementById('newServerUrl').value = '';
      document.getElementById('newServerSecret').value = '';
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
    const res = await fetch(`/api/proxy/api/files?path=${encodeURIComponent(path)}`);
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
    // Sort directories first
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
    const res = await fetch('/api/proxy/api/files/mkdir', {
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

async function uploadSelectedFile() {
  const input = document.getElementById('fileUploadInput');
  if (!input.files || input.files.length === 0) return;
  const file = input.files[0];

  const formData = new FormData();
  formData.append('file', file);
  formData.append('dir', currentFilesPath);

  try {
    const res = await fetch('/api/proxy/api/files/upload', {
      method: 'POST',
      body: formData
    });
    if (res.ok) {
      input.value = '';
      loadFiles(currentFilesPath);
    } else {
      alert('Ошибка загрузки файла');
    }
  } catch (e) {
    alert('Ошибка: ' + e.message);
  }
}

function downloadFile(filePath) {
  window.open(`/api/proxy/api/files/download?path=${encodeURIComponent(filePath)}`, '_blank');
}

async function shareFile(filePath) {
  try {
    const res = await fetch('/api/proxy/api/files/share', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: filePath })
    });
    if (!res.ok) {
      const err = await res.json();
      alert('Ошибка: ' + err.error);
      return;
    }
    const data = await res.json();
    const remoteUrl = activeServer ? activeServer.url.replace(/\/$/, '') : window.location.origin;
    const directShareUrl = `${remoteUrl}/share/${data.token}`;
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
    const res = await fetch(`/api/proxy/api/files/view?path=${encodeURIComponent(filePath)}`);
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
    const res = await fetch('/api/proxy/api/files/save', {
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
    const res = await fetch('/api/proxy/api/files/delete', {
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
}

function switchDbSubTab(tab) {
  currentDbSubTab = tab;
  document.getElementById('subTabKV').className = 'tab-btn ' + (tab === 'kv' ? 'active' : '');
  document.getElementById('subTabSQL').className = 'tab-btn ' + (tab === 'sql' ? 'active' : '');

  document.getElementById('panelDbKV').style.display = tab === 'kv' ? 'block' : 'none';
  document.getElementById('panelDbSQL').style.display = tab === 'sql' ? 'block' : 'none';
}

function getDbRequestPayload() {
  return {
    driver: currentDbDriver,
    host: document.getElementById('dbHostInput').value.trim() || 'localhost',
    port: parseInt(document.getElementById('dbPortInput').value) || (currentDbDriver === 'postgres' ? 5432 : 3306),
    user: document.getElementById('dbUserInput').value.trim(),
    password: document.getElementById('dbPassInput').value,
    db_name: document.getElementById('dbNameInput').value.trim() || (currentDbDriver === 'postgres' ? 'postgres' : 'mysql'),
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
    const res = await fetch('/api/proxy/api/db/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if (res.ok) {
      if (!silent) alert('Успешно: ' + (data.message || 'Соединение установлено!'));
      closeModal('modalDbConfig');
    } else {
      if (!silent) alert('Ошибка соединения: ' + data.error);
    }
  } catch (e) {
    if (!silent) alert('Ошибка: ' + e.message);
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
    const res = await fetch('/api/proxy/api/db/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if (res.ok) {
      alert(data.message || 'База успешно создана!');
      document.getElementById('dbNameInput').value = name;
      closeModal('modalCreateDb');
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
    const res = await fetch('/api/proxy/api/db/kv/read', {
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
    const res = await fetch('/api/proxy/api/db/kv/write', {
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
    const res = await fetch('/api/proxy/api/db/kv/delete', {
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
    const res = await fetch('/api/proxy/api/db/query', {
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

function setScriptLang(lang) {
  currentScriptLang = lang;
  document.getElementById('langPy').className = 'tab-btn ' + (lang === 'python' ? 'active' : '');
  document.getElementById('langNode').className = 'tab-btn ' + (lang === 'nodejs' ? 'active' : '');
  document.getElementById('langGo').className = 'tab-btn ' + (lang === 'golang' ? 'active' : '');

  const ext = lang === 'python' ? '.py' : (lang === 'nodejs' ? '.js' : '.go');
  document.getElementById('btnScriptActionText').innerText = `Запустить ${ext}`;

  loadCurrentScript();
}

async function loadCurrentScript() {
  const id = document.getElementById('scriptIdInput').value.trim() || 'main';
  try {
    const res = await fetch(`/api/proxy/api/scripts/get?id=${encodeURIComponent(id)}`);
    if (res.ok) {
      const data = await res.json();
      if (data.content) {
        document.getElementById('scriptContentInput').value = data.content;
      }
    }
  } catch (e) {
    console.error(e);
  }
}

function loadScriptFromFile(event) {
  const file = event.target.files[0];
  if (!file) return;

  const reader = new FileReader();
  reader.onload = function(e) {
    document.getElementById('scriptContentInput').value = e.target.result;
    if (file.name.endsWith('.py')) setScriptLang('python');
    else if (file.name.endsWith('.js') || file.name.endsWith('.ts')) setScriptLang('nodejs');
    else if (file.name.endsWith('.go')) setScriptLang('golang');
  };
  reader.readAsText(file);
}

async function runScript() {
  const id = document.getElementById('scriptIdInput').value.trim() || 'main';
  const content = document.getElementById('scriptContentInput').value;
  const autoDeps = document.getElementById('scriptAutoDeps').checked;

  const terminal = document.getElementById('scriptLogsTerminal');
  const badge = document.getElementById('scriptStatusBadge');
  terminal.innerText = 'Запуск скрипта...\n';
  badge.className = 'log-status-badge running';
  badge.innerText = 'Запуск...';

  try {
    const res = await fetch('/api/proxy/api/scripts/run', {
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
      terminal.innerText += `Ошибка: ${err.error}\n`;
      badge.className = 'log-status-badge';
      badge.innerText = 'Ошибка';
      return;
    }

    startScriptLogStream(id);
  } catch (e) {
    terminal.innerText += `Ошибка запроса: ${e.message}\n`;
    badge.className = 'log-status-badge';
    badge.innerText = 'Ошибка';
  }
}

async function stopScript() {
  const id = document.getElementById('scriptIdInput').value.trim() || 'main';
  try {
    await fetch('/api/proxy/api/scripts/stop', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id })
    });
    document.getElementById('scriptStatusBadge').className = 'log-status-badge';
    document.getElementById('scriptStatusBadge').innerText = 'Остановлен';
  } catch (e) {
    console.error(e);
  }
}

async function fetchScriptLogs() {
  const id = document.getElementById('scriptIdInput').value.trim() || 'main';
  try {
    const res = await fetch(`/api/proxy/api/scripts/logs?id=${encodeURIComponent(id)}`);
    if (res.ok) {
      const data = await res.json();
      document.getElementById('scriptLogsTerminal').innerText = data.logs || 'Логи пусты.';
    }
  } catch (e) {
    console.error(e);
  }
}

function startScriptLogStream(id) {
  if (scriptLogSource) {
    scriptLogSource.close();
  }

  const terminal = document.getElementById('scriptLogsTerminal');
  const badge = document.getElementById('scriptStatusBadge');

  const sseUrl = `/api/proxy/api/scripts/logs/stream?id=${encodeURIComponent(id)}`;
  scriptLogSource = new EventSource(sseUrl);

  scriptLogSource.addEventListener('init', (e) => {
    terminal.innerText = e.data ? e.data.replace(/\\n/g, '\n') : '';
    terminal.scrollTop = terminal.scrollHeight;
  });

  scriptLogSource.onmessage = (e) => {
    try {
      const data = JSON.parse(e.data);
      if (data.log) {
        terminal.innerText += data.log;
        terminal.scrollTop = terminal.scrollHeight;
      }
      if (data.status) {
        badge.innerText = data.status === 'running' ? 'Выполняется' : (data.status === 'installing_deps' ? 'Зависимости...' : data.status);
        badge.className = 'log-status-badge ' + (data.status === 'running' ? 'running' : '');
      }
    } catch (_) {
      terminal.innerText += e.data + '\n';
      terminal.scrollTop = terminal.scrollHeight;
    }
  };

  scriptLogSource.onerror = () => {
    scriptLogSource.close();
  };
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
