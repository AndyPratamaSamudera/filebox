import { auth } from '../auth.js';
import {
  api, uploadItem, uploadByUrl, itemUrl, getUploadConfig, createChunkSession, uploadChunk, completeChunkSession,
  search, listItems, getItemDetail, createFolder, updateItem, deleteItem, listSharedItems, listFavorites,
  listApiKeys, createApiKey, revokeApiKey, revealApiKey,
} from '../api.js';
import { iconSvg, fileIconName } from '../icons.js';
import { passwordInputHtml, bindPasswordToggles } from '../ui.js';
import { chunkRanges } from '../crypto.js';

function escapeHtml(str) {
  return String(str).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

function hasExtension(name) {
  const extStart = name.lastIndexOf('.');
  return extStart > 0 && extStart < name.length - 1;
}

export async function renderDashboard(container, onLogout) {
  const state = {
    currentDirectory: '',
    items: [],
    favoriteItems: [],
    sharedItems: [],
    friends: [],
    friendRequests: [],
    friendSearchQuery: '',
    friendsTab: 'friends',
    loading: false,
    error: '',
    uploading: false,
    view: 'files',
    uploads: [],
    searchQuery: '',
    uploadConfig: null,
    searchResults: null,
    pendingFiles: [],
    uploadModal: false,
    uploadFavorite: false,
    uploadShare: false,
    uploadShareRecipients: new Set(),
    uploadPassword: '',
    uploadUrlModal: false,
    uploadUrl: '',
    uploadUrlFavorite: false,
    uploadUrlShare: false,
    uploadUrlShareRecipients: new Set(),
    uploadUrlPassword: '',
    speedDialOpen: false,
    contextMenu: null,
    createFolderModal: false,
    createFolderName: '',
    editItem: null,
    editItemName: '',
    lockItem: null,
    lockPassword: '',
    shareItem: null,
    shareRecipients: new Set(),
    itemShares: [],
    unlockItem: null,
    unlockAction: '',
    unlockPassword: '',
    apiKeys: [],
    newApiKey: null,
    revealKey: null,
    addFriendModal: false,
    addFriendEmail: '',
    selectedItemPaths: new Set(),
    bulkActionModal: null,
    bulkShareRecipients: new Set(),
    bulkPassword: '',
  };

  const uploadMaxDirect = () => state.uploadConfig?.upload_max_direct ?? 10 * 1024 * 1024;
  const chunkSize = () => state.uploadConfig?.chunk_size ?? 10 * 1024 * 1024;

  function mount() {
    container.innerHTML = layoutHtml();
    bindLayoutEvents();
    update();
    init();
    window.addEventListener('resize', update);
  }

  function layoutHtml() {
    return `
      <div class="app">
        <aside class="sidebar">
          <div class="brand"><span class="brand-logo">${iconSvg('box', 22)}</span><span class="brand-name">FileBox</span></div>
          <nav class="nav">
            <button class="nav-item" data-view="files">${iconSvg('home', 18)}<span>My Files</span></button>
            <button class="nav-item" data-view="favorites">${iconSvg('heart', 18)}<span>Favorites</span></button>
            <button class="nav-item" data-view="shared">${iconSvg('share', 18)}<span>Shared with me</span></button>
            <button class="nav-item" data-view="friends">${iconSvg('users', 18)}<span>Friends</span></button>
            <button class="nav-item" data-view="profile">${iconSvg('user', 18)}<span>Profile</span></button>
          </nav>
        </aside>
        <main class="main">
          <header class="page-header">
            <div class="header-left"><h1 class="page-title" id="page-title">My Files</h1><span class="item-count" id="item-count">0 items</span></div>
            <div class="header-right"></div>
          </header>
          <div id="breadcrumb" class="breadcrumb hidden"></div>
          <div id="alert-container" class="alert-container"></div>
          <div id="upload-modal" class="modal-backdrop hidden"></div>
          <div id="upload-url-modal" class="modal-backdrop hidden"></div>
          <div id="create-folder-modal" class="modal-backdrop hidden"></div>
          <div id="edit-item-modal" class="modal-backdrop hidden"></div>
          <div id="share-modal" class="modal-backdrop hidden"></div>
          <div id="lock-modal" class="modal-backdrop hidden"></div>
          <div id="unlock-modal" class="modal-backdrop hidden"></div>
          <div id="add-friend-modal" class="modal-backdrop hidden"></div>
          <div id="reveal-key-modal" class="modal-backdrop hidden"></div>
          <div id="bulk-action-modal" class="modal-backdrop hidden"></div>
          <div id="context-menu" class="context-menu hidden"></div>
          <div id="toast-container" class="toast-container"></div>
          <section class="content" id="content"></section>
          <nav class="bottom-nav">
            <button class="bottom-nav-item" data-view="files">${iconSvg('home', 20)}<span>Files</span></button>
            <button class="bottom-nav-item" data-view="favorites">${iconSvg('heart', 20)}<span>Favorites</span></button>
            <button class="bottom-nav-item" data-view="shared">${iconSvg('share', 20)}<span>Shared</span></button>
            <button class="bottom-nav-item" data-view="friends">${iconSvg('users', 20)}<span>Friends</span></button>
            <button class="bottom-nav-item" data-view="profile">${iconSvg('user', 20)}<span>Profile</span></button>
          </nav>
        </main>
      </div>
    `;
  }

  function bindLayoutEvents() {
    container.querySelectorAll('.nav-item[data-view]').forEach((btn) => {
      btn.addEventListener('click', () => setView(btn.dataset.view));
    });
    container.querySelectorAll('.bottom-nav-item[data-view]').forEach((btn) => {
      btn.addEventListener('click', () => setView(btn.dataset.view));
    });
    container.querySelector('#create-folder-btn')?.addEventListener('click', () => openCreateFolder());
    container.querySelector('#upload-btn')?.addEventListener('click', () => openUploadModal());
    container.addEventListener('click', (e) => {
      if (state.contextMenu && !e.target.closest('.context-menu')) closeContextMenu();
    });
    const searchInput = container.querySelector('#search-input');
    if (searchInput) {
      searchInput.addEventListener('input', (e) => { state.searchQuery = e.target.value; update(); });
      searchInput.addEventListener('keydown', async (e) => {
        if (e.key === 'Enter') {
          const q = e.target.value.trim();
          if (!q) return;
          state.view = 'search';
          await runSearch(q);
        }
      });
    }
    const friendSearchInput = container.querySelector('#friend-search-input');
    if (friendSearchInput) {
      friendSearchInput.addEventListener('input', (e) => { state.friendSearchQuery = e.target.value.trim().toLowerCase(); update(); });
    }
  }

  async function init() {
    try {
      await loadFriends();
      state.uploadConfig = await getUploadConfig();
      container.querySelector('#upload-btn').disabled = false;
      await load();
    } catch (e) {
      if (e.message.includes('session')) logout();
      else { state.error = e.message; update(); }
    }
  }

  async function load() {
    state.loading = true;
    state.error = '';
    update();
    try {
      if (state.view === 'files') {
        state.items = await listItems(state.currentDirectory);
        state.favoriteItems = [];
        state.sharedItems = [];
        state.searchResults = null;
      } else if (state.view === 'favorites') {
        state.items = [];
        state.favoriteItems = await listFavorites();
        state.sharedItems = [];
        state.searchResults = null;
      } else if (state.view === 'shared') {
        state.items = [];
        state.favoriteItems = [];
        state.sharedItems = await listSharedItems();
        state.searchResults = null;
      } else if (state.view === 'friends') {
        state.items = [];
        state.favoriteItems = [];
        state.sharedItems = [];
        state.searchResults = null;
        await loadFriends();
      } else if (state.view === 'profile') {
        state.items = [];
        state.favoriteItems = [];
        state.sharedItems = [];
        state.searchResults = null;
        state.apiKeys = await listApiKeys();
      }
    } catch (e) {
      state.error = e.message;
    } finally {
      state.loading = false;
      update();
    }
  }

  async function runSearch(q) {
    state.loading = true;
    state.error = '';
    state.searchQuery = q;
    update();
    try {
      state.searchResults = await search(q);
    } catch (e) {
      state.error = e.message;
      state.searchResults = null;
    } finally {
      state.loading = false;
      update();
    }
  }

  function setView(view) {
    state.view = view;
    state.searchResults = null;
    state.selectedItemPaths = new Set();
    if (view !== 'files') state.currentDirectory = '';
    load();
  }

  function goUp() {
    if (state.currentDirectory === '') return;
    const idx = state.currentDirectory.lastIndexOf('/');
    state.currentDirectory = idx === -1 ? '' : state.currentDirectory.slice(0, idx);
    state.selectedItemPaths = new Set();
    load();
  }

  function openFolder(item) {
    if (item.type !== 'folder') return;
    state.currentDirectory = item.path;
    state.selectedItemPaths = new Set();
    load();
  }

  async function loadFriends() {
    try {
      const [friends, requests] = await Promise.all([
        api('/friends').catch(() => []),
        api('/friends/requests').catch(() => []),
      ]);
      state.friends = friends ?? [];
      state.friendRequests = requests ?? [];
    } catch {
      state.friends = [];
      state.friendRequests = [];
    }
  }

  function openAddFriend() { state.addFriendModal = true; state.addFriendEmail = ''; update(); }
  function isMobile() { return window.innerWidth <= 768; }
  function toggleSpeedDial() { state.speedDialOpen = !state.speedDialOpen; update(); }
  function closeSpeedDial() { state.speedDialOpen = false; update(); }
  function openContextMenu(item, x, y) { state.contextMenu = { item, x, y }; update(); }
  function closeContextMenu() { state.contextMenu = null; update(); }
  function closeAddFriend() { state.addFriendModal = false; state.addFriendEmail = ''; update(); }
  async function addFriend() {
    const email = state.addFriendEmail.trim();
    if (!email) return;
    try {
      await api('/friends', { method: 'POST', body: { email } });
      await loadFriends();
      closeAddFriend();
      showSuccess('Friend request sent');
      update();
    } catch (e) { state.error = e.message; update(); }
  }
  async function removeFriend(f) {
    if (!confirm('Remove this friend?')) return;
    try { await api(`/friends/${f.id}`, { method: 'DELETE' }); await loadFriends(); update(); }
    catch (e) { state.error = e.message; update(); }
  }
  function setFriendsTab(tab) { state.friendsTab = tab; update(); }
  async function acceptRequest(req) {
    try { await api(`/friends/requests/${req.id}/accept`, { method: 'POST' }); await loadFriends(); update(); }
    catch (e) { state.error = e.message; update(); }
  }
  async function rejectRequest(req) {
    try { await api(`/friends/requests/${req.id}/reject`, { method: 'POST' }); await loadFriends(); update(); }
    catch (e) { state.error = e.message; update(); }
  }
  async function cancelRequest(req) {
    try { await api(`/friends/requests/${req.id}`, { method: 'DELETE' }); await loadFriends(); update(); }
    catch (e) { state.error = e.message; update(); }
  }

  async function logout() {
    try { await api('/auth/logout', { method: 'POST', body: { refresh_token: auth.refresh } }); } catch {}
    auth.clear();
    onLogout();
  }

  async function createNewFolder() {
    const name = state.createFolderName.trim();
    if (!name) return;
    const directory = state.currentDirectory ? `${state.currentDirectory}/${name}` : name;
    try {
      await createFolder(directory);
      state.createFolderModal = false;
      state.createFolderName = '';
      await load();
      showSuccess('Folder created');
    } catch (e) { state.error = e.message; update(); }
  }

  async function renameItem(item) {
    const name = state.editItemName.trim();
    if (!name) return;
    try {
      await updateItem(item.path, { name });
      state.editItem = null;
      state.editItemName = '';
      await load();
      showSuccess('Renamed');
    } catch (e) { state.error = e.message; update(); }
  }

  async function toggleFavorite(item) {
    if (item.type !== 'file') return;
    try {
      await updateItem(item.path, { is_favorite: !item.is_favorite });
      await load();
    } catch (e) { state.error = e.message; update(); }
  }

  async function deleteItemByPath(item) {
    const noun = item.type === 'folder' ? 'folder and its contents' : 'file';
    if (!confirm(`Delete this ${noun} permanently? This cannot be undone.`)) return;
    try { await deleteItem(item.path); await load(); }
    catch (e) { state.error = e.message; update(); }
  }

  async function saveLockPassword() {
    const item = state.lockItem;
    if (!item) return;
    const password = state.lockPassword.trim();
    try {
      await updateItem(item.path, { password });
      state.lockItem = null;
      state.lockPassword = '';
      await load();
      showSuccess(password ? 'Password set' : 'Password removed');
    } catch (e) { state.error = e.message; update(); }
  }

  async function saveShare() {
    const item = state.shareItem;
    if (!item) return;
    try {
      await updateItem(item.path, { shares: Array.from(state.shareRecipients) });
      state.shareItem = null;
      state.shareRecipients = new Set();
      state.itemShares = [];
      await load();
      showSuccess('Shares updated');
    } catch (e) { state.error = e.message; update(); }
  }

  async function saveBulkPassword() {
    const selected = (state.items ?? []).filter((it) => it.type === 'file' && state.selectedItemPaths.has(it.path));
    const password = state.bulkPassword.trim();
    if (selected.length === 0) { closeBulkActionModal(); return; }
    try {
      for (const item of selected) { await updateItem(item.path, { password }); }
      state.selectedItemPaths = new Set();
      closeBulkActionModal();
      showSuccess(password ? 'Password set' : 'Password removed');
      await load();
    } catch (e) { state.error = e.message; update(); }
  }

  async function saveBulkShare() {
    const selected = (state.items ?? []).filter((it) => it.type === 'file' && state.selectedItemPaths.has(it.path));
    const emails = Array.from(state.bulkShareRecipients);
    if (selected.length === 0 || emails.length === 0) { closeBulkActionModal(); return; }
    try {
      for (const item of selected) { await updateItem(item.path, { shares: emails }); }
      state.selectedItemPaths = new Set();
      closeBulkActionModal();
      showSuccess('Shared selected files');
      await load();
    } catch (e) { state.error = e.message; update(); }
  }

  async function deleteSelectedItems() {
    const paths = Array.from(state.selectedItemPaths);
    if (paths.length === 0) return;
    if (!confirm(`Delete ${paths.length} item${paths.length === 1 ? '' : 's'} permanently? This cannot be undone.`)) return;
    try {
      for (const p of paths) { await deleteItem(p); }
      state.selectedItemPaths = new Set();
      await load();
    } catch (e) { state.error = e.message; update(); }
  }

  async function bulkFavorite() {
    const selected = (state.items ?? []).filter((it) => it.type === 'file' && state.selectedItemPaths.has(it.path));
    if (selected.length === 0) return;
    const target = selected.every((it) => it.is_favorite) ? false : true;
    try {
      for (const item of selected) { await updateItem(item.path, { is_favorite: target }); }
      state.selectedItemPaths = new Set();
      await load();
      showSuccess(`${target ? 'Starred' : 'Unstarred'} ${selected.length} file${selected.length === 1 ? '' : 's'}`);
    } catch (e) { state.error = e.message; update(); }
  }

  function bulkDownload() {
    const selected = (state.items ?? []).filter((it) => it.type === 'file' && state.selectedItemPaths.has(it.path));
    for (const item of selected) { try { downloadItem(item, false); } catch {} }
  }

  function downloadItem(item, preview = false) {
    const path = item.path || item.item_path;
    if (!path) return;
    if (item.is_locked || item.item_is_locked) {
      openUnlockModal(item, preview ? 'preview' : 'download');
      return;
    }
    openFileUrl(path, preview ? 'preview' : 'download', '', item.name || item.item_name);
  }

  function openFileUrl(path, kind, password = '', filename = '') {
    const url = itemUrl(path, kind, password);
    const a = document.createElement('a');
    a.href = url;
    a.target = '_blank';
    if (kind === 'download') a.download = filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
  }

  function openUnlockModal(item, action) {
    state.unlockItem = item;
    state.unlockAction = action;
    state.unlockPassword = '';
    update();
  }

  function closeUnlockModal() {
    state.unlockItem = null;
    state.unlockAction = '';
    state.unlockPassword = '';
    update();
  }

  function confirmUnlock() {
    const item = state.unlockItem;
    const action = state.unlockAction;
    const password = state.unlockPassword.trim();
    if (!item || !action) { closeUnlockModal(); return; }
    closeUnlockModal();
    openFileUrl(item.path || item.item_path, action === 'preview' ? 'preview' : 'download', password, item.name || item.item_name);
  }

  function openUploadModal() {
    state.uploadModal = true;
    state.pendingFiles = [];
    state.uploadFavorite = false;
    state.uploadShare = false;
    state.uploadShareRecipients = new Set();
    state.uploadPassword = '';
    update();
  }
  function closeUploadModal() { state.uploadModal = false; state.pendingFiles = []; state.uploadShareRecipients = new Set(); state.uploadPassword = ''; update(); }
  function openUploadUrlModal() {
    state.uploadUrlModal = true;
    state.uploadUrl = '';
    state.uploadUrlFavorite = false;
    state.uploadUrlShare = false;
    state.uploadUrlShareRecipients = new Set();
    state.uploadUrlPassword = '';
    update();
  }
  function closeUploadUrlModal() { state.uploadUrlModal = false; state.uploadUrl = ''; state.uploadUrlShareRecipients = new Set(); state.uploadUrlPassword = ''; update(); }
  function openCreateFolder() { state.createFolderModal = true; state.createFolderName = ''; update(); }
  function closeCreateFolder() { state.createFolderModal = false; state.createFolderName = ''; update(); }
  function openEditItem(item) {
    const name = item.name || item.item_name || '';
    const ext = item.type === 'file' ? name.slice(name.lastIndexOf('.')) : '';
    state.editItem = item;
    state.editItemName = item.type === 'file' ? name.slice(0, name.length - ext.length) : name;
    update();
  }
  function closeEditItem() { state.editItem = null; state.editItemName = ''; update(); }
  function openLockModal(item) { state.lockItem = item; state.lockPassword = ''; update(); }
  function closeLockModal() { state.lockItem = null; state.lockPassword = ''; update(); }
  async function openShareModal(item) {
    state.shareItem = item;
    state.shareRecipients = new Set();
    state.itemShares = [];
    try {
      const detail = await getItemDetail(item.path);
      state.itemShares = detail.shares ?? [];
      detail.shares?.forEach((s) => state.shareRecipients.add(s.shared_email));
    } catch (e) { state.error = e.message; }
    update();
  }
  function closeShareModal() { state.shareItem = null; state.shareRecipients = new Set(); state.itemShares = []; update(); }
  function openBulkActionModal(mode) { state.bulkActionModal = mode; state.bulkShareRecipients = new Set(); state.bulkPassword = ''; update(); }
  function closeBulkActionModal() { state.bulkActionModal = null; state.bulkShareRecipients = new Set(); state.bulkPassword = ''; update(); }

  function selectAllItems(checked) {
    const files = (state.items ?? []).filter((it) => it.type === 'file');
    if (checked) files.forEach((it) => state.selectedItemPaths.add(it.path));
    else files.forEach((it) => state.selectedItemPaths.delete(it.path));
    update();
  }

  function startUpload() {
    const files = [...state.pendingFiles];
    if (files.length === 0) return;
    state.uploading = true;
    state.uploadModal = false;
    update();
    files.forEach((file) => uploadSingleFile(file, state.currentDirectory));
  }

  async function uploadFromUrl() {
    const urls = state.uploadUrl.split('\n').map((u) => u.trim()).filter(Boolean);
    if (urls.length === 0) return;
    state.uploadUrlModal = false;
    state.uploading = true;
    const shareWith = state.uploadUrlShare ? Array.from(state.uploadUrlShareRecipients) : [];
    const baseMeta = {
      directory: state.currentDirectory,
      favorite: state.uploadUrlFavorite,
      password: state.uploadUrlPassword,
      share_with: shareWith,
    };
    const entries = urls.map((url) => ({
      id: Math.random().toString(36).slice(2),
      url,
      name: url.split('/').pop() || 'URL download',
      progress: 0,
      done: false,
      error: '',
    }));
    state.uploads.push(...entries);
    update();
    for (const entry of entries) {
      try {
        await uploadByUrl({ url: entry.url, ...baseMeta });
        state.uploads = state.uploads.map((u) => u.id === entry.id ? { ...u, progress: 100, done: true } : u);
      } catch (e) {
        state.uploads = state.uploads.map((u) => u.id === entry.id ? { ...u, error: e.message } : u);
      }
      update();
    }
    await load();
    const failed = entries.some((entry) => state.uploads.find((u) => u.id === entry.id)?.error);
    if (!failed) showSuccess('Files downloaded from URL');
    state.uploading = state.uploads.some((u) => !u.done && !u.error);
    state.uploadUrl = '';
    state.uploadUrlPassword = '';
    state.uploadUrlShareRecipients = new Set();
    update();
    setTimeout(() => {
      state.uploads = state.uploads.filter((u) => !entries.find((entry) => entry.id === u.id));
      update();
    }, 3000);
  }

  async function uploadSingleFile(file, directory) {
    const id = Math.random().toString(36).slice(2);
    state.uploads.push({ id, name: file.name, progress: 0, done: false, error: '' });
    update();
    try {
      const meta = {
        favorite: state.uploadFavorite,
        password: state.uploadPassword,
        share_with: state.uploadShare ? Array.from(state.uploadShareRecipients).join(',') : '',
      };
      const max = uploadMaxDirect();
      if (file.size > max) {
        await uploadChunkedFile(file, directory, meta, id);
      } else {
        await uploadItem(file, directory, meta);
      }
      state.uploads = state.uploads.map((u) => u.id === id ? { ...u, progress: 100, done: true } : u);
      await load();
    } catch (e) {
      state.uploads = state.uploads.map((u) => u.id === id ? { ...u, error: e.message } : u);
    } finally {
      state.uploading = state.uploads.some((u) => !u.done && !u.error);
      update();
      setTimeout(() => { state.uploads = state.uploads.filter((u) => u.id !== id); update(); }, 3000);
    }
  }

  async function uploadChunkedFile(file, directory, meta, uploadId) {
    if (!hasExtension(file.name)) {
      throw new Error('Only files with an extension can be uploaded');
    }
    const cs = chunkSize();
    const totalSize = file.size;
    const totalChunks = Math.ceil(totalSize / cs);
    const ext = file.name.slice(file.name.lastIndexOf('.'));
    const init = await createChunkSession({
      directory,
      name: file.name,
      ext,
      mime: file.type || 'application/octet-stream',
      password: meta.password || '',
      chunk_size: cs,
      total_chunks: totalChunks,
      total_size: totalSize,
    });
    const ranges = chunkRanges(totalSize, cs);
    for (let i = 0; i < ranges.length; i++) {
      const { start, end } = ranges[i];
      const chunk = file.slice(start, end);
      let lastError = null;
      for (let attempt = 0; attempt < 3; attempt++) {
        try {
          await uploadChunk(init.upload_id, i, chunk);
          lastError = null;
          break;
        } catch (e) {
          lastError = e;
          if (attempt < 2) await new Promise((r) => setTimeout(r, 1000 * (attempt + 1)));
        }
      }
      if (lastError) throw new Error(`chunk ${i + 1}/${ranges.length} failed: ${lastError.message}`);
      const pct = Math.min(99, Math.round(((i + 1) / ranges.length) * 100));
      state.uploads = state.uploads.map((u) => u.id === uploadId ? { ...u, progress: pct } : u);
      update();
    }
    await completeChunkSession(init.upload_id);
  }

  function handleFileSelection(e) {
    const files = [...(e.target.files || [])];
    const rejected = [];
    const accepted = [];
    for (const file of files) {
      if (hasExtension(file.name)) accepted.push(file);
      else rejected.push(file.name);
    }
    if (rejected.length) {
      const list = rejected.slice(0, 3).join(', ') + (rejected.length > 3 ? ` and ${rejected.length - 3} more` : '');
      showError(`Only files with an extension can be uploaded. Skipped: ${list}`);
    }
    if (accepted.length) { state.pendingFiles = [...state.pendingFiles, ...accepted]; update(); }
  }

  function update() {
    try {
      updateSafe();
    } catch (err) {
      console.error('Dashboard update failed:', err);
      state.error = `UI error: ${err.message}`;
      const alertEl = container.querySelector('#alert-container');
      if (alertEl) alertEl.innerHTML = `<div class="alert-toast alert-error"><span>${escapeHtml(state.error)}</span></div>`;
    }
  }

  let lastView = null;
  function updateSafe() {
    const used = auth.user?.storage_used || 0;
    const quota = auth.user?.storage_quota || auth.user?.total_storage || 0;
    const pct = quota ? Math.min(100, Math.round((used / quota) * 100)) : 0;
    const fill = container.querySelector('#storage-fill');
    const text = container.querySelector('#storage-text');
    if (fill) fill.style.width = `${pct}%`;
    if (text) {
      const usedStr = `${fmtSize(used)} used`;
      text.textContent = quota ? `${usedStr} · ${pct}% of ${fmtSize(quota)}` : `${usedStr} · Unlimited`;
    }
    if (lastView !== state.view) {
      renderHeader();
      lastView = state.view;
    }
    container.querySelectorAll('.nav-item[data-view]').forEach((btn) => {
      btn.classList.toggle('active', btn.dataset.view === state.view);
    });
    container.querySelectorAll('.bottom-nav-item[data-view]').forEach((btn) => {
      btn.classList.toggle('active', btn.dataset.view === state.view);
    });

    const uploadBtn = container.querySelector('#upload-btn');
    if (uploadBtn) uploadBtn.disabled = !state.uploadConfig;

    const pageTitleEl = container.querySelector('#page-title');
    const itemCountEl = container.querySelector('#item-count');
    const titles = { files: 'My Files', favorites: 'Favorites', shared: 'Shared with me', friends: 'Friends', profile: 'Profile', search: `Search: ${state.searchQuery}` };
    if (pageTitleEl) pageTitleEl.textContent = titles[state.view] || 'My Files';
    let count = 0;
    if (state.view === 'search') count = state.searchResults?.length ?? 0;
    else if (state.view === 'favorites') count = (state.favoriteItems ?? []).length;
    else if (state.view === 'shared') count = (state.sharedItems ?? []).length;
    else if (state.view === 'friends') count = (state.friends ?? []).length;
    else if (state.view === 'profile') count = null;
    else count = (state.items ?? []).length;
    if (itemCountEl) itemCountEl.textContent = count === null ? '' : `${count} items`;

    renderBreadcrumb();
    renderAlert();
    renderUploadModal();
    renderUploadUrlModal();
    renderCreateFolderModal();
    renderEditItemModal();
    renderShareModal();
    renderLockModal();
    renderUnlockModal();
    renderAddFriendModal();
    renderRevealKeyModal();
    renderBulkActionModal();
    renderContextMenu();
    renderToasts();
    renderContent();
  }

  function renderHeader() {
    const el = container.querySelector('.header-right');
    if (!el) return;
    const searchItem = ['files', 'favorites', 'shared', 'search'].includes(state.view) ? `
      <div class="search-wrap">${iconSvg('search', 16)}<input class="search-input" id="search-input" type="text" value="${escapeHtml(state.searchQuery)}" placeholder="Search items…" autocomplete="off" /></div>
    ` : '';
    const friendSearch = state.view === 'friends' ? `
      <div class="search-wrap">${iconSvg('search', 16)}<input class="search-input" id="friend-search-input" type="text" value="${escapeHtml(state.friendSearchQuery)}" placeholder="Find friends…" autocomplete="off" /></div>
    ` : '';
    const headerBtns = state.view === 'files' ? `
      <div class="header-btns">
        <button class="btn btn-ghost" id="create-folder-btn" title="New Folder">${iconSvg('plus', 16)}<span class="btn-label">New Folder</span></button>
        <button class="btn btn-primary" id="upload-btn" title="Upload">${iconSvg('upload', 16)}<span class="btn-label">Upload</span></button>
      </div>
    ` : '';
    el.innerHTML = searchItem + friendSearch + headerBtns;

    const searchInput = el.querySelector('#search-input');
    if (searchInput) {
      searchInput.addEventListener('input', (e) => { state.searchQuery = e.target.value; update(); });
      searchInput.addEventListener('keydown', async (e) => {
        if (e.key === 'Enter') {
          const q = e.target.value.trim();
          if (!q) return;
          state.view = 'search';
          await runSearch(q);
        }
      });
    }
    const friendSearchInput = el.querySelector('#friend-search-input');
    if (friendSearchInput) {
      friendSearchInput.addEventListener('input', (e) => { state.friendSearchQuery = e.target.value.trim().toLowerCase(); update(); });
    }
    const uploadBtn = el.querySelector('#upload-btn');
    if (uploadBtn) {
      uploadBtn.disabled = !state.uploadConfig;
      uploadBtn.addEventListener('click', () => openUploadModal());
    }
    const createFolderBtn = el.querySelector('#create-folder-btn');
    if (createFolderBtn) createFolderBtn.addEventListener('click', () => openCreateFolder());
  }

  function renderBreadcrumb() {
    const el = container.querySelector('#breadcrumb');
    if (!el) return;
    if (state.view !== 'files' || state.currentDirectory === '') { el.classList.add('hidden'); return; }
    el.classList.remove('hidden');
    const parts = state.currentDirectory.split('/');
    let html = `<button class="crumb-back">Home</button>`;
    let path = '';
    for (const seg of parts) {
      path = path ? `${path}/${seg}` : seg;
      html += `<span class="crumb-sep">/</span><button class="crumb-part" data-path="${escapeHtml(path)}">${escapeHtml(seg)}</button>`;
    }
    el.innerHTML = html;
    el.querySelector('.crumb-back').addEventListener('click', goUp);
    el.querySelectorAll('.crumb-part').forEach((btn) => {
      btn.addEventListener('click', () => { state.currentDirectory = btn.dataset.path; load(); });
    });
  }

  function renderContent() {
    const el = container.querySelector('#content');
    if (!el) return;
    if (state.loading) { el.innerHTML = `<div class="skeleton-table"><div class="skeleton-head"></div>${Array(6).fill('<div class="skeleton-row"></div>').join('')}</div>`; return; }
    if (state.view === 'friends') { renderFriends(el); return; }
    if (state.view === 'profile') { renderProfile(el); return; }

    let items = [];
    if (state.view === 'search') items = state.searchResults ?? [];
    else if (state.view === 'favorites') items = state.favoriteItems ?? [];
    else if (state.view === 'shared') items = state.sharedItems ?? [];
    else items = state.items ?? [];

    const isFilesView = state.view === 'files';
    const isSearch = state.view === 'search';
    const selectable = isFilesView || state.view === 'favorites';
    const useCards = isMobile() && (isFilesView || isSearch || state.view === 'favorites' || state.view === 'shared');
    const selectedFiles = items.filter((it) => it.type === 'file' && state.selectedItemPaths.has(it.path || it.item_path));
    const allFav = selectedFiles.length > 0 && selectedFiles.every((it) => it.is_favorite);
    const bulkBar = !useCards && selectable && state.selectedItemPaths.size > 0 ? `
      <div class="bulk-bar card">
        <span>${state.selectedItemPaths.size} selected</span>
        <div class="bulk-actions">
          <button class="btn btn-sm btn-ghost" data-action="bulk-favorite" title="${allFav ? 'Unstar' : 'Star'} selected">${iconSvg(allFav ? 'star' : 'starOff', 14)}</button>
          <button class="btn btn-sm btn-ghost" data-action="bulk-download" title="Download selected">${iconSvg('download', 14)}</button>
          <button class="btn btn-sm btn-ghost" data-action="bulk-lock" title="Set password">${iconSvg('lock', 14)}</button>
          <button class="btn btn-sm btn-ghost" data-action="bulk-share" title="Share selected">${iconSvg('share', 14)}</button>
          <button class="btn btn-sm btn-danger" data-action="bulk-delete">${iconSvg('trash', 14)} Delete</button>
        </div>
      </div>
    ` : '';

    if (items.length === 0) {
      const emptyTitle = state.view === 'favorites' ? 'No favorites' : state.view === 'shared' ? 'No shared files' : state.view === 'search' ? 'No results' : 'Nothing here yet';
      const emptyMsg = state.view === 'favorites' ? 'Star files to see them here.' : state.view === 'shared' ? 'Friends have not shared any files with you yet.' : state.view === 'search' ? `No items match "${escapeHtml(state.searchQuery)}".` : state.currentDirectory ? 'This folder is empty.' : 'Upload a file or create a folder to get started.';
      el.innerHTML = bulkBar + emptyStateHtml(emptyTitle, emptyMsg);
      return;
    }

    if (useCards) {
      el.innerHTML = `
        ${bulkBar}
        <div class="file-card-grid">
          ${renderCards(items, isFilesView)}
        </div>
      `;
      bindCardEvents(el, items);
      return;
    }

    el.innerHTML = `
      ${bulkBar}
      <div class="table-wrap card ${selectable ? 'selectable' : ''}">
        <div class="table-head">
          ${selectable ? `<span class="table-check"><input type="checkbox" id="select-all-items" ${items.filter((it) => it.type === 'file').length > 0 && items.filter((it) => it.type === 'file').every((it) => state.selectedItemPaths.has(it.path || it.item_path)) ? 'checked' : ''} /></span>` : ''}
          <span>Name</span><span>Size</span><span>Date</span><span></span>
        </div>
        ${renderRows(items, isFilesView, selectable)}
      </div>
    `;
    bindRowEvents(el, items);
  }

  function renderRows(items, isFilesView, selectable) {
    return items.map((item) => {
      const isFolder = item.type === 'folder';
      const isShared = state.view === 'shared';
      const name = item.name || item.item_name || '';
      const size = isFolder ? '—' : fmtSize(item.size || item.item_size || 0);
      const date = fmtDate(item.created_at);
      const path = item.path || item.item_path || '';
      const check = selectable && !isFolder ? `<span class="table-check"><input type="checkbox" class="select-item" data-path="${escapeHtml(path)}" ${state.selectedItemPaths.has(path) ? 'checked' : ''} /></span>` : (selectable ? '<span class="table-check"></span>' : '');
      const actionCell = isShared ? `
        <div class="table-actions">
          <span class="muted owner">by ${escapeHtml(item.owner_name)}</span>
          <button class="action-btn" data-action="download-item" title="Download">${iconSvg('download', 16)}</button>
        </div>` : `
        <div class="table-actions">
          <button class="action-btn" data-action="rename-item" title="Rename">${iconSvg('edit', 16)}</button>
          ${isFolder ? '' : `<button class="action-btn ${item.is_favorite ? 'active' : ''}" data-action="toggle-fav" title="${item.is_favorite ? 'Unstar' : 'Star'}">${iconSvg(item.is_favorite ? 'star' : 'starOff', 16)}</button>`}
          ${isFolder ? '' : `<button class="action-btn ${item.share_count ? 'active' : ''}" data-action="share-item" title="${item.share_count ? `Shared with ${item.share_count} friend${item.share_count === 1 ? '' : 's'}` : 'Share'}">${iconSvg('share', 16)}</button>`}
          ${isFolder ? '' : `<button class="action-btn ${item.is_locked ? 'active' : ''}" data-action="lock-item" title="${item.is_locked ? 'Change password' : 'Set password'}">${iconSvg('lock', 16)}</button>`}
          ${isFolder ? '' : `<button class="action-btn" data-action="download-item" title="Download">${iconSvg('download', 16)}</button>`}
          <button class="action-btn danger" data-action="delete-item" title="Delete">${iconSvg('trash', 16)}</button>
        </div>
      `;
      const rowClass = isFolder ? 'folder-row' : 'file-row';
      const icon = isFolder ? iconSvg('folder', 18) : iconSvg(fileIconName(item), 18);
      const isLocked = item.is_locked || item.item_is_locked;
      const nameBtn = isFolder ? `<button class="table-cell table-name open-folder">${icon}<span class="name-text">${escapeHtml(name)}</span></button>`
        : `<button class="table-cell table-name preview-item">${icon}<span class="name-text">${escapeHtml(name)}</span>${isLocked ? `<span class="lock-badge" title="Password protected">${iconSvg('lock', 12)}</span>` : ''}${item.share_count ? `<span class="shared-badge" title="Shared with ${item.share_count} friend${item.share_count === 1 ? '' : 's'}">${iconSvg('share', 12)}</span>` : ''}</button>`;
      return `<div class="table-row ${rowClass}" data-path="${escapeHtml(path)}">${check}${nameBtn}<span class="table-cell muted">${size}</span><span class="table-cell muted">${date}</span>${actionCell}</div>`;
    }).join('');
  }

  function bindRowEvents(el, items) {
    el.querySelectorAll('.open-folder').forEach((btn) => {
      const path = btn.closest('.table-row').dataset.path;
      const item = items.find((it) => (it.path || it.item_path) === path);
      btn.addEventListener('click', () => openFolder(item));
    });
    el.querySelectorAll('.preview-item').forEach((btn) => {
      const path = btn.closest('.table-row').dataset.path;
      const item = items.find((it) => (it.path || it.item_path) === path);
      btn.addEventListener('click', () => downloadItem(item, true));
    });
    el.querySelectorAll('[data-action="rename-item"]').forEach((btn) => {
      const path = btn.closest('.table-row').dataset.path;
      const item = items.find((it) => (it.path || it.item_path) === path);
      btn.addEventListener('click', () => openEditItem(item));
    });
    el.querySelectorAll('[data-action="toggle-fav"]').forEach((btn) => {
      const path = btn.closest('.table-row').dataset.path;
      const item = items.find((it) => (it.path || it.item_path) === path);
      btn.addEventListener('click', () => toggleFavorite(item));
    });
    el.querySelectorAll('[data-action="share-item"]').forEach((btn) => {
      const path = btn.closest('.table-row').dataset.path;
      const item = items.find((it) => (it.path || it.item_path) === path);
      btn.addEventListener('click', () => openShareModal(item));
    });
    el.querySelectorAll('[data-action="lock-item"]').forEach((btn) => {
      const path = btn.closest('.table-row').dataset.path;
      const item = items.find((it) => (it.path || it.item_path) === path);
      btn.addEventListener('click', () => openLockModal(item));
    });
    el.querySelectorAll('[data-action="download-item"]').forEach((btn) => {
      const path = btn.closest('.table-row').dataset.path;
      const item = items.find((it) => (it.path || it.item_path) === path);
      btn.addEventListener('click', () => downloadItem(item, false));
    });
    el.querySelectorAll('[data-action="delete-item"]').forEach((btn) => {
      const path = btn.closest('.table-row').dataset.path;
      const item = items.find((it) => (it.path || it.item_path) === path);
      btn.addEventListener('click', () => deleteItemByPath(item));
    });
    el.querySelectorAll('.select-item').forEach((cb) => {
      cb.addEventListener('change', (e) => {
        if (e.target.checked) state.selectedItemPaths.add(e.target.dataset.path);
        else state.selectedItemPaths.delete(e.target.dataset.path);
        update();
      });
    });
    el.querySelector('#select-all-items')?.addEventListener('change', (e) => selectAllItems(e.target.checked));
    el.querySelectorAll('[data-action="bulk-favorite"]').forEach((b) => b.addEventListener('click', bulkFavorite));
    el.querySelectorAll('[data-action="bulk-download"]').forEach((b) => b.addEventListener('click', bulkDownload));
    el.querySelectorAll('[data-action="bulk-lock"]').forEach((b) => b.addEventListener('click', () => openBulkActionModal('password')));
    el.querySelectorAll('[data-action="bulk-share"]').forEach((b) => b.addEventListener('click', () => openBulkActionModal('share')));
    el.querySelectorAll('[data-action="bulk-delete"]').forEach((b) => b.addEventListener('click', deleteSelectedItems));
  }

  function renderCards(items) {
    return items.map((item) => {
      const isFolder = item.type === 'folder';
      const name = item.name || item.item_name || '';
      const icon = isFolder ? iconSvg('folder', 40) : iconSvg(fileIconName(item), 40);
      return `
        <div class="file-card ${isFolder ? 'folder-card' : 'file-card'}" data-path="${escapeHtml(item.path || item.item_path || '')}" tabindex="0">
          <div class="file-card-icon">${icon}</div>
          <div class="file-card-name">${escapeHtml(name)}</div>
        </div>
      `;
    }).join('');
  }

  function bindCardEvents(el, items) {
    el.querySelectorAll('.file-card').forEach((card) => {
      const path = card.dataset.path;
      const item = items.find((it) => (it.path || it.item_path) === path);
      if (!item) return;

      let longPressTimer = null;
      let longPressTriggered = false;
      const startLongPress = (e) => {
        longPressTriggered = false;
        longPressTimer = setTimeout(() => {
          longPressTriggered = true;
          const evt = e.touches ? e.touches[0] : e;
          openContextMenu(item, evt.clientX, evt.clientY);
        }, 500);
      };
      const cancelLongPress = () => {
        if (longPressTimer) { clearTimeout(longPressTimer); longPressTimer = null; }
      };

      card.addEventListener('touchstart', startLongPress, { passive: true });
      card.addEventListener('touchend', cancelLongPress);
      card.addEventListener('touchmove', cancelLongPress);
      card.addEventListener('touchcancel', cancelLongPress);
      card.addEventListener('mousedown', startLongPress);
      card.addEventListener('mouseup', cancelLongPress);
      card.addEventListener('mouseleave', cancelLongPress);
      card.addEventListener('contextmenu', (e) => { e.preventDefault(); openContextMenu(item, e.clientX, e.clientY); });
      card.addEventListener('click', (e) => {
        if (longPressTriggered) { e.preventDefault(); e.stopPropagation(); return; }
        if (item.type === 'folder') openFolder(item);
        else downloadItem(item, true);
      });
    });
  }

  function renderContextMenu() {
    const el = container.querySelector('#context-menu');
    if (!el) return;
    if (!state.contextMenu) { el.classList.add('hidden'); el.innerHTML = ''; return; }
    el.classList.remove('hidden');
    const { item, x, y } = state.contextMenu;
    const isFolder = item.type === 'folder';
    const isShared = state.view === 'shared';
    const name = item.name || item.item_name || '';

    const menuItems = [];
    if (isFolder) {
      menuItems.push({ action: 'open', label: 'Open', icon: 'folder' });
      menuItems.push({ action: 'rename', label: 'Rename', icon: 'edit' });
      menuItems.push({ action: 'delete', label: 'Delete', icon: 'trash', danger: true });
    } else if (isShared) {
      menuItems.push({ action: 'download', label: 'Download', icon: 'download' });
      menuItems.push({ action: 'owner', label: `Owner: ${escapeHtml(item.owner_name || '')}`, icon: 'user', disabled: true });
    } else {
      menuItems.push({ action: 'preview', label: 'Preview', icon: 'eye' });
      menuItems.push({ action: 'download', label: 'Download', icon: 'download' });
      menuItems.push({ action: 'rename', label: 'Rename', icon: 'edit' });
      menuItems.push({ action: 'favorite', label: item.is_favorite ? 'Remove from favorites' : 'Add to favorites', icon: item.is_favorite ? 'starOff' : 'star' });
      menuItems.push({ action: 'share', label: item.share_count ? `Shared (${item.share_count})` : 'Share', icon: 'share' });
      menuItems.push({ action: 'lock', label: item.is_locked ? 'Change password' : 'Set password', icon: 'lock' });
      menuItems.push({ action: 'delete', label: 'Delete', icon: 'trash', danger: true });
    }

    el.innerHTML = `
      <div class="context-menu-head">${escapeHtml(name)}</div>
      ${menuItems.map((m) => `
        <button class="context-menu-item ${m.danger ? 'danger' : ''}" data-action="${m.action}" ${m.disabled ? 'disabled' : ''}>
          ${iconSvg(m.icon, 18)}<span>${m.label}</span>
        </button>
      `).join('')}
    `;

    // Position the menu inside the viewport.
    const rect = el.getBoundingClientRect();
    const menuWidth = rect.width || 200;
    const menuHeight = rect.height || 250;
    let left = Math.min(x, window.innerWidth - menuWidth - 8);
    let top = Math.min(y, window.innerHeight - menuHeight - 8);
    if (left < 8) left = 8;
    if (top < 8) top = 8;
    el.style.left = `${left}px`;
    el.style.top = `${top}px`;

    el.querySelectorAll('.context-menu-item').forEach((btn) => {
      btn.addEventListener('click', () => {
        const action = btn.dataset.action;
        closeContextMenu();
        if (action === 'open' || action === 'preview') { if (isFolder) openFolder(item); else downloadItem(item, action === 'preview'); }
        else if (action === 'download') downloadItem(item, false);
        else if (action === 'rename') openEditItem(item);
        else if (action === 'favorite') toggleFavorite(item);
        else if (action === 'share') openShareModal(item);
        else if (action === 'lock') openLockModal(item);
        else if (action === 'delete') deleteItemByPath(item);
      });
    });
  }

  function renderSearchResults(el) {
    const items = state.searchResults || [];
    if (items.length === 0) { el.innerHTML = emptyStateHtml('No results', `No items match "${escapeHtml(state.searchQuery)}".`); return; }
    el.innerHTML = `
      <div class="table-wrap card">
        <div class="table-head"><span>Name</span><span>Size</span><span>Date</span><span></span></div>
        ${renderRows(items, false, false)}
      </div>
    `;
    bindRowEvents(el, items);
  }

  function renderFriends(el) {
    const q = state.friendSearchQuery;
    const matchesFriend = (f) => {
      if (!q) return true;
      return (f.friend_name || '').toLowerCase().includes(q) || (f.friend_email || '').toLowerCase().includes(q);
    };
    const matchesRequest = (r) => {
      if (!q) return true;
      return (r.requester_name || r.recipient_name || '').toLowerCase().includes(q) || (r.requester_email || r.recipient_email || '').toLowerCase().includes(q);
    };
    const friends = (state.friends ?? []).filter(matchesFriend);
    const incomingAll = state.friendRequests.filter((r) => r.status === 'pending' && r.recipient_user_id === auth.user?.id);
    const outgoingAll = state.friendRequests.filter((r) => r.status === 'pending' && r.requester_user_id === auth.user?.id);
    const incoming = incomingAll.filter(matchesRequest);
    const outgoing = outgoingAll.filter(matchesRequest);
    const incomingCount = incomingAll.length;
    el.innerHTML = `
      <div class="card friends-view">
        <div class="friends-view-head"><h3>Friends</h3><button class="btn btn-primary" id="open-add-friend">${iconSvg('plus', 16)}<span>Add friend</span></button></div>
        <div class="tabs">
          <button class="tab ${state.friendsTab === 'friends' ? 'active' : ''}" data-tab="friends">Friends ${(state.friends ?? []).length > 0 ? `(${(state.friends ?? []).length})` : ''}</button>
          <button class="tab ${state.friendsTab === 'requests' ? 'active' : ''}" data-tab="requests">Requests ${incomingCount > 0 ? `<span class="badge">${incomingCount}</span>` : ''}</button>
        </div>
        ${state.friendsTab === 'friends' ? `
          ${friends.length === 0 ? `<p class="muted">${q ? 'No friends match your search.' : 'No friends yet.'}</p>` : `
            <div class="friend-list">
              ${friends.map((f) => `
                <div class="friend-row">
                  <span>${escapeHtml(f.friend_name)} <span class="muted">(${escapeHtml(f.friend_email)})</span></span>
                  <button class="action-btn danger" data-remove="${f.id}">${iconSvg('trash', 14)}</button>
                </div>
              `).join('')}
            </div>
          `}
        ` : `
          ${incoming.length === 0 && outgoing.length === 0 ? `<p class="muted">${q ? 'No requests match your search.' : 'No pending requests.'}</p>` : ''}
          ${incoming.length > 0 ? `
            <div class="request-section">
              <label class="muted small">Incoming</label>
              <div class="friend-list">
                ${incoming.map((r) => `
                  <div class="friend-row">
                    <span>${escapeHtml(r.requester_name)} <span class="muted">(${escapeHtml(r.requester_email)})</span></span>
                    <div class="friend-actions">
                      <button class="btn btn-sm btn-primary" data-accept="${r.id}">Accept</button>
                      <button class="btn btn-sm btn-ghost" data-reject="${r.id}">Reject</button>
                    </div>
                  </div>
                `).join('')}
              </div>
            </div>
          ` : ''}
          ${outgoing.length > 0 ? `
            <div class="request-section">
              <label class="muted small">Outgoing</label>
              <div class="friend-list">
                ${outgoing.map((r) => `
                  <div class="friend-row">
                    <span>${escapeHtml(r.recipient_name)} <span class="muted">(${escapeHtml(r.recipient_email)})</span></span>
                    <button class="btn btn-sm btn-ghost" data-cancel="${r.id}">Cancel</button>
                  </div>
                `).join('')}
              </div>
            </div>
          ` : ''}
        `}
      </div>
    `;
    el.querySelector('#open-add-friend')?.addEventListener('click', () => openAddFriend());
    el.querySelectorAll('[data-tab]').forEach((btn) => btn.addEventListener('click', () => setFriendsTab(btn.dataset.tab)));
    el.querySelectorAll('[data-remove]').forEach((btn) => btn.addEventListener('click', () => {
      const f = (state.friends ?? []).find((x) => String(x.id) === btn.dataset.remove);
      if (f) removeFriend(f);
    }));
    el.querySelectorAll('[data-accept]').forEach((btn) => btn.addEventListener('click', () => {
      const r = state.friendRequests.find((x) => String(x.id) === btn.dataset.accept);
      if (r) acceptRequest(r);
    }));
    el.querySelectorAll('[data-reject]').forEach((btn) => btn.addEventListener('click', () => {
      const r = state.friendRequests.find((x) => String(x.id) === btn.dataset.reject);
      if (r) rejectRequest(r);
    }));
    el.querySelectorAll('[data-cancel]').forEach((btn) => btn.addEventListener('click', () => {
      const r = state.friendRequests.find((x) => String(x.id) === btn.dataset.cancel);
      if (r) cancelRequest(r);
    }));
  }

  function renderProfile(el) {
    const used = auth.user?.storage_used || 0;
    const quota = auth.user?.storage_quota || auth.user?.total_storage || 0;
    const pct = quota ? Math.min(100, Math.round((used / quota) * 100)) : 0;
    el.innerHTML = `
      <div class="card profile-view">
        <div class="profile-head">
          <div class="profile-avatar">${iconSvg('user', 32)}</div>
          <div class="profile-meta">
            <div class="profile-name">${escapeHtml(auth.user?.username || 'User')}</div>
            <div class="profile-email muted">${escapeHtml(auth.user?.email || '')}</div>
          </div>
        </div>
        <div class="profile-storage">
          <div class="profile-storage-title">
            <span class="storage-title">${iconSvg('hardDrive', 14)}<span>Storage</span></span>
            <span class="storage-text">${quota ? `${fmtSize(used)} / ${fmtSize(quota)} (${pct}%)` : `${fmtSize(used)} used`}</span>
          </div>
          <div class="storage-track"><div class="storage-fill" style="width:${pct}%"></div></div>
        </div>
        <div id="profile-api-keys"></div>
        <div class="profile-actions">
          <button class="btn btn-danger" id="profile-logout">${iconSvg('logOut', 16)}<span>Logout</span></button>
        </div>
      </div>
    `;
    el.querySelector('#profile-logout')?.addEventListener('click', () => logout());
    const keysEl = el.querySelector('#profile-api-keys');
    if (keysEl) renderApiKeys(keysEl);
  }

  function renderApiKeys(el) {
    let keyNotice = '';
    if (state.newApiKey) {
      keyNotice = `
        <div class="alert alert-warning api-key-notice">
          <p><strong>Copy this key now — it will not be shown again.</strong></p>
          <code class="api-key-code">${escapeHtml(state.newApiKey)}</code>
          <button class="btn btn-ghost btn-sm" id="dismiss-key">Dismiss</button>
        </div>
      `;
    }
    const rows = (state.apiKeys ?? []).map((k) => `
      <div class="api-key-row">
        <div>
          <div class="name-text">${escapeHtml(k.name)}</div>
          <div class="muted small">Created ${fmtDate(k.created_at)}${k.last_used_at ? ' · Last used ' + fmtDate(k.last_used_at) : ''}</div>
        </div>
        <div class="api-key-actions">
          <button class="action-btn" data-action="show-key" data-key-id="${k.id}" data-key-name="${escapeHtml(k.name)}" title="Show key">${iconSvg('eye', 16)}</button>
          <button class="action-btn danger" data-action="revoke-key" data-key-id="${k.id}" title="Revoke">${iconSvg('trash', 16)}</button>
        </div>
      </div>
    `).join('');
    el.innerHTML = `
      <div class="card api-keys">
        <div class="api-keys-head"><h3>API Keys</h3><button class="btn btn-primary" id="create-api-key">${iconSvg('plus', 16)}<span>New Key</span></button></div>
        ${keyNotice}
        ${rows ? `<div class="api-key-list">${rows}</div>` : '<p class="muted">No API keys yet.</p>'}
      </div>
    `;
    el.querySelector('#create-api-key')?.addEventListener('click', async () => {
      const name = prompt('API key name');
      if (!name) return;
      try {
        const res = await createApiKey({ name });
        state.newApiKey = res.key;
        await load();
      } catch (e) { state.error = e.message; update(); }
    });
    el.querySelector('#dismiss-key')?.addEventListener('click', () => { state.newApiKey = null; update(); });
    el.querySelectorAll('[data-action="revoke-key"]').forEach((btn) => {
      const id = Number(btn.dataset.keyId);
      btn.addEventListener('click', async () => {
        if (!confirm('Revoke this API key?')) return;
        try { await revokeApiKey(id); await load(); }
        catch (e) { state.error = e.message; update(); }
      });
    });
    el.querySelectorAll('[data-action="show-key"]').forEach((btn) => {
      const id = Number(btn.dataset.keyId);
      const name = btn.dataset.keyName || '';
      btn.addEventListener('click', () => {
        state.revealKey = { id, name, password: '', key: '', error: '' };
        update();
      });
    });
  }

  function renderRevealKeyModal() {
    const el = container.querySelector('#reveal-key-modal');
    if (!el) return;
    if (!state.revealKey) { el.classList.add('hidden'); el.innerHTML = ''; return; }
    el.classList.remove('hidden');
    const { id, name, password, key, error } = state.revealKey;
    el.innerHTML = `
      <div class="modal card reveal-key-modal">
        <div class="modal-head"><h3>Reveal API Key</h3><button class="action-btn modal-close" data-close="reveal-key" aria-label="Close">${iconSvg('close', 16)}</button></div>
        <div class="modal-body">
          <p class="muted">Enter your account password to view the key for <strong>${escapeHtml(name)}</strong>.</p>
          ${key ? `
            <div class="form-group">
              <label class="form-label">API Key</label>
              <div class="api-key-code-wrap"><code class="api-key-code">${escapeHtml(key)}</code></div>
              <button class="btn btn-ghost btn-sm" id="copy-revealed-key">Copy</button>
            </div>
          ` : `
            <div class="form-group">
              ${passwordInputHtml({ id: 'reveal-key-password', label: 'Account Password', placeholder: 'Your account password', required: true, name: 'reveal-key-password' })}
            </div>
            ${error ? `<div class="alert alert-error">${escapeHtml(error)}</div>` : ''}
            <div class="modal-actions">
              <button class="btn btn-ghost" data-close="reveal-key">Cancel</button>
              <button class="btn btn-primary" id="confirm-reveal-key">Reveal</button>
            </div>
          `}
        </div>
      </div>
    `;
    bindPasswordToggles(el);
    el.querySelector('[data-close="reveal-key"]')?.addEventListener('click', () => { state.revealKey = null; update(); });
    el.querySelector('#copy-revealed-key')?.addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(key);
        showError('Copied to clipboard');
      } catch {
        showError('Failed to copy');
      }
    });
    const input = el.querySelector('#reveal-key-password');
    input?.addEventListener('input', (e) => { if (state.revealKey) state.revealKey.password = e.target.value; });
    input?.addEventListener('keydown', (e) => { if (e.key === 'Enter') el.querySelector('#confirm-reveal-key')?.click(); });
    el.querySelector('#confirm-reveal-key')?.addEventListener('click', async () => {
      if (!state.revealKey) return;
      const { id: keyId, password: pwd } = state.revealKey;
      try {
        const res = await revealApiKey(keyId, pwd);
        state.revealKey.key = res.key || res.data?.key || '';
        state.revealKey.error = '';
      } catch (e) {
        state.revealKey.error = e.message || 'Failed to reveal key';
      }
      update();
    });
  }

  function renderUploadModal() {
    const el = container.querySelector('#upload-modal');
    if (!el) return;
    if (!state.uploadModal) { el.classList.add('hidden'); el.innerHTML = ''; return; }
    el.classList.remove('hidden');
    const hasFiles = state.pendingFiles.length > 0;
    const fileList = hasFiles ? `
      <div class="upload-modal-files">
        <h4>${state.pendingFiles.length} file${state.pendingFiles.length === 1 ? '' : 's'} selected</h4>
        <div class="upload-modal-list">
          ${state.pendingFiles.map((file) => `
            <div class="upload-modal-row">
              <span class="name-text">${escapeHtml(file.name)}</span>
              <span class="muted small">${fmtSize(file.size)}</span>
            </div>
          `).join('')}
        </div>
      </div>
    ` : `
      <div class="upload-drop-zone" id="upload-drop-zone">
        <div class="upload-drop-icon">${iconSvg('upload', 32)}</div>
        <p class="muted">Drag & drop files here</p>
        <button class="btn btn-ghost" id="choose-files-btn">Choose files</button>
        <input type="file" id="modal-file-input" multiple hidden />
      </div>
    `;
    el.innerHTML = `
      <div class="modal upload-modal card">
        <div class="modal-head"><h3>Upload Files</h3><button class="action-btn modal-close" id="close-upload-modal" aria-label="Close">${iconSvg('close', 16)}</button></div>
        <div class="modal-body">
          ${fileList}
          <div class="upload-modal-options">
            <label class="upload-option-row">
              <input type="checkbox" id="upload-favorite" ${state.uploadFavorite ? 'checked' : ''} />
              <span>Add to favorites</span>
            </label>
            <label class="upload-option-row">
              <input type="checkbox" id="upload-share-toggle" ${state.uploadShare ? 'checked' : ''} />
              <span>Share with friends</span>
            </label>
            ${state.uploadShare ? `
              <div class="upload-share-friends">
                ${(state.friends ?? []).length > 0 ? (state.friends ?? []).map((f) => `
                  <label class="share-friend-row">
                    <input type="checkbox" value="${escapeHtml(f.friend_email)}" ${state.uploadShareRecipients.has(f.friend_email) ? 'checked' : ''} />
                    <span>${escapeHtml(f.friend_name)} <span class="muted">(${escapeHtml(f.friend_email)})</span></span>
                  </label>
                `).join('') : '<p class="muted small">No friends yet.</p>'}
              </div>
            ` : ''}
            <div class="upload-modal-password">
              <label class="muted small">File password (optional)</label>
              ${passwordInputHtml({ id: 'upload-password', placeholder: 'Leave empty for no lock', value: state.uploadPassword })}
            </div>
            <button class="btn btn-sm btn-ghost upload-url-link" id="open-upload-url-modal" type="button">${iconSvg('link', 16)} Upload from URL</button>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn btn-ghost" id="cancel-upload">Cancel</button>
          <button class="btn btn-primary" id="confirm-upload" ${hasFiles ? '' : 'disabled'}>Upload</button>
        </div>
      </div>
    `;
    el.querySelector('#close-upload-modal').addEventListener('click', closeUploadModal);
    el.querySelector('#cancel-upload').addEventListener('click', closeUploadModal);
    el.querySelector('#confirm-upload').addEventListener('click', startUpload);
    el.querySelector('#open-upload-url-modal')?.addEventListener('click', () => { closeUploadModal(); openUploadUrlModal(); });
    const favInput = el.querySelector('#upload-favorite');
    if (favInput) favInput.addEventListener('change', (e) => { state.uploadFavorite = e.target.checked; update(); });
    const shareToggle = el.querySelector('#upload-share-toggle');
    if (shareToggle) shareToggle.addEventListener('change', (e) => { state.uploadShare = e.target.checked; update(); });
    bindPasswordToggles(el);
    const pwInput = el.querySelector('#upload-password');
    if (pwInput) pwInput.addEventListener('input', (e) => { state.uploadPassword = e.target.value; });
    el.querySelectorAll('.upload-share-friends .share-friend-row input').forEach((cb) => {
      cb.addEventListener('change', (e) => {
        if (e.target.checked) state.uploadShareRecipients.add(e.target.value);
        else state.uploadShareRecipients.delete(e.target.value);
        update();
      });
    });
    const chooseBtn = el.querySelector('#choose-files-btn');
    const modalInput = el.querySelector('#modal-file-input');
    if (chooseBtn && modalInput) {
      chooseBtn.addEventListener('click', () => modalInput.click());
      modalInput.addEventListener('change', handleFileSelection);
    }
    const dropZone = el.querySelector('#upload-drop-zone');
    if (dropZone) {
      dropZone.addEventListener('dragover', (e) => { e.preventDefault(); dropZone.classList.add('drag-over'); });
      dropZone.addEventListener('dragleave', () => dropZone.classList.remove('drag-over'));
      dropZone.addEventListener('drop', (e) => {
        e.preventDefault();
        dropZone.classList.remove('drag-over');
        const files = [...(e.dataTransfer?.files || [])];
        if (files.length) { state.pendingFiles = [...state.pendingFiles, ...files]; update(); }
      });
    }
    el.addEventListener('click', (e) => { if (e.target === el) closeUploadModal(); });
  }

  function renderUploadUrlModal() {
    const el = container.querySelector('#upload-url-modal');
    if (!el) return;
    if (!state.uploadUrlModal) { el.classList.add('hidden'); el.innerHTML = ''; return; }
    el.classList.remove('hidden');
    el.innerHTML = `
      <div class="modal upload-url-modal card">
        <div class="modal-head"><h3>Upload from URL</h3><button class="action-btn modal-close" id="close-upload-url-modal" aria-label="Close">${iconSvg('close', 16)}</button></div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">File URLs</label>
            <textarea id="upload-url-input" class="input" rows="4" placeholder="https://example.com/file.pdf&#10;https://example.com/image.jpg">${escapeHtml(state.uploadUrl)}</textarea>
          </div>
          <div class="upload-modal-options">
            <label class="upload-option-row">
              <input type="checkbox" id="upload-url-favorite" ${state.uploadUrlFavorite ? 'checked' : ''} />
              <span>Add to favorites</span>
            </label>
            <label class="upload-option-row">
              <input type="checkbox" id="upload-url-share-toggle" ${state.uploadUrlShare ? 'checked' : ''} />
              <span>Share with friends</span>
            </label>
            ${state.uploadUrlShare ? `
              <div class="upload-share-friends">
                ${(state.friends ?? []).length > 0 ? (state.friends ?? []).map((f) => `
                  <label class="share-friend-row">
                    <input type="checkbox" value="${escapeHtml(f.friend_email)}" ${state.uploadUrlShareRecipients.has(f.friend_email) ? 'checked' : ''} />
                    <span>${escapeHtml(f.friend_name)} <span class="muted">(${escapeHtml(f.friend_email)})</span></span>
                  </label>
                `).join('') : '<p class="muted small">No friends yet.</p>'}
              </div>
            ` : ''}
            <div class="upload-modal-password">
              <label class="muted small">File password (optional)</label>
              ${passwordInputHtml({ id: 'upload-url-password', placeholder: 'Leave empty for no lock', value: state.uploadUrlPassword })}
            </div>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn btn-ghost" id="cancel-upload-url">Cancel</button>
          <button class="btn btn-primary" id="confirm-upload-url">Download</button>
        </div>
      </div>
    `;
    el.querySelector('#close-upload-url-modal').addEventListener('click', closeUploadUrlModal);
    el.querySelector('#cancel-upload-url').addEventListener('click', closeUploadUrlModal);
    el.querySelector('#confirm-upload-url').addEventListener('click', uploadFromUrl);
    const urlInput = el.querySelector('#upload-url-input');
    urlInput.focus();
    urlInput.addEventListener('input', (e) => { state.uploadUrl = e.target.value; });
    const favInput = el.querySelector('#upload-url-favorite');
    if (favInput) favInput.addEventListener('change', (e) => { state.uploadUrlFavorite = e.target.checked; update(); });
    const shareToggle = el.querySelector('#upload-url-share-toggle');
    if (shareToggle) shareToggle.addEventListener('change', (e) => { state.uploadUrlShare = e.target.checked; update(); });
    bindPasswordToggles(el);
    const pwInput = el.querySelector('#upload-url-password');
    if (pwInput) pwInput.addEventListener('input', (e) => { state.uploadUrlPassword = e.target.value; });
    el.querySelectorAll('.upload-share-friends .share-friend-row input').forEach((cb) => {
      cb.addEventListener('change', (e) => {
        if (e.target.checked) state.uploadUrlShareRecipients.add(e.target.value);
        else state.uploadUrlShareRecipients.delete(e.target.value);
        update();
      });
    });
    el.addEventListener('click', (e) => { if (e.target === el) closeUploadUrlModal(); });
  }

  function renderCreateFolderModal() {
    const el = container.querySelector('#create-folder-modal');
    if (!el) return;
    if (!state.createFolderModal) { el.classList.add('hidden'); el.innerHTML = ''; return; }
    el.classList.remove('hidden');
    el.innerHTML = `
      <div class="modal card">
        <div class="modal-head"><h3>New Folder</h3><button class="action-btn modal-close" id="close-create-folder" aria-label="Close">${iconSvg('close', 16)}</button></div>
        <div class="modal-body">
          <label class="muted small form-label">Folder name</label>
          <input type="text" id="folder-name-input" class="input" value="${escapeHtml(state.createFolderName)}" placeholder="e.g. documents" />
        </div>
        <div class="modal-actions">
          <button class="btn btn-ghost" id="cancel-create-folder">Cancel</button>
          <button class="btn btn-primary" id="confirm-create-folder">Create</button>
        </div>
      </div>
    `;
    const input = el.querySelector('#folder-name-input');
    input.focus();
    input.addEventListener('input', (e) => { state.createFolderName = e.target.value; });
    input.addEventListener('keydown', (e) => { if (e.key === 'Enter') createNewFolder(); });
    el.querySelector('#close-create-folder').addEventListener('click', closeCreateFolder);
    el.querySelector('#cancel-create-folder').addEventListener('click', closeCreateFolder);
    el.querySelector('#confirm-create-folder').addEventListener('click', createNewFolder);
    el.addEventListener('click', (e) => { if (e.target === el) closeCreateFolder(); });
  }

  function renderEditItemModal() {
    const el = container.querySelector('#edit-item-modal');
    if (!el) return;
    if (!state.editItem) { el.classList.add('hidden'); el.innerHTML = ''; return; }
    el.classList.remove('hidden');
    const noun = state.editItem.type === 'folder' ? 'folder' : 'file';
    el.innerHTML = `
      <div class="modal card">
        <div class="modal-head"><h3>Rename ${noun}</h3><button class="action-btn modal-close" id="close-edit-item" aria-label="Close">${iconSvg('close', 16)}</button></div>
        <div class="modal-body">
          <label class="muted small form-label">Name</label>
          <input type="text" id="edit-item-name" class="input" value="${escapeHtml(state.editItemName)}" placeholder="new name" />
          ${state.editItem.type === 'file' ? '<p class="muted small">Extension is preserved automatically.</p>' : ''}
        </div>
        <div class="modal-actions">
          <button class="btn btn-ghost" id="cancel-edit-item">Cancel</button>
          <button class="btn btn-primary" id="confirm-edit-item">Save</button>
        </div>
      </div>
    `;
    const input = el.querySelector('#edit-item-name');
    input.focus();
    input.addEventListener('input', (e) => { state.editItemName = e.target.value; });
    input.addEventListener('keydown', (e) => { if (e.key === 'Enter') renameItem(state.editItem); });
    el.querySelector('#close-edit-item').addEventListener('click', closeEditItem);
    el.querySelector('#cancel-edit-item').addEventListener('click', closeEditItem);
    el.querySelector('#confirm-edit-item').addEventListener('click', () => renameItem(state.editItem));
    el.addEventListener('click', (e) => { if (e.target === el) closeEditItem(); });
  }

  function renderShareModal() {
    const el = container.querySelector('#share-modal');
    if (!el) return;
    if (!state.shareItem) { el.classList.add('hidden'); el.innerHTML = ''; return; }
    el.classList.remove('hidden');
    const currentEmails = Array.from(state.shareRecipients);
    const friendOptions = (state.friends ?? []).map((f) => `
      <label class="share-friend-row">
        <input type="checkbox" value="${escapeHtml(f.friend_email)}" ${state.shareRecipients.has(f.friend_email) ? 'checked' : ''} />
        <span>${escapeHtml(f.friend_name)} <span class="muted">(${escapeHtml(f.friend_email)})</span></span>
      </label>
    `).join('');
    el.innerHTML = `
      <div class="modal card">
        <div class="modal-head"><h3>Share ${escapeHtml(state.shareItem.name)}</h3><button class="action-btn modal-close" id="close-share-modal" aria-label="Close">${iconSvg('close', 16)}</button></div>
        <div class="modal-body">
          <div class="share-friend-list">
            ${(state.friends ?? []).length > 0 ? friendOptions : '<p class="muted small">No friends yet.</p>'}
          </div>
          <div class="share-add-email">
            <input type="email" id="share-email-input" class="input" placeholder="friend@example.com" />
            <button class="btn btn-ghost" id="add-share-email">Add</button>
          </div>
          ${currentEmails.length > 0 ? `<p class="muted small">Sharing with: ${currentEmails.map(escapeHtml).join(', ')}</p>` : ''}
        </div>
        <div class="modal-actions">
          <button class="btn btn-ghost" id="cancel-share">Cancel</button>
          <button class="btn btn-primary" id="confirm-share">Save</button>
        </div>
      </div>
    `;
    el.querySelectorAll('.share-friend-row input').forEach((cb) => {
      cb.addEventListener('change', (e) => {
        if (e.target.checked) state.shareRecipients.add(e.target.value);
        else state.shareRecipients.delete(e.target.value);
        update();
      });
    });
    el.querySelector('#add-share-email').addEventListener('click', () => {
      const email = el.querySelector('#share-email-input').value.trim().toLowerCase();
      if (email) { state.shareRecipients.add(email); update(); }
    });
    el.querySelector('#close-share-modal').addEventListener('click', closeShareModal);
    el.querySelector('#cancel-share').addEventListener('click', closeShareModal);
    el.querySelector('#confirm-share').addEventListener('click', saveShare);
    el.addEventListener('click', (e) => { if (e.target === el) closeShareModal(); });
  }

  function renderLockModal() {
    const el = container.querySelector('#lock-modal');
    if (!el) return;
    if (!state.lockItem) { el.classList.add('hidden'); el.innerHTML = ''; return; }
    el.classList.remove('hidden');
    const locked = state.lockItem.is_locked;
    el.innerHTML = `
      <div class="modal card">
        <div class="modal-head"><h3>${locked ? 'Change Password' : 'Set Password'}</h3><button class="action-btn modal-close" id="close-lock-modal" aria-label="Close">${iconSvg('close', 16)}</button></div>
        <div class="modal-body">
          <p class="muted small">${locked ? 'Enter a new password to replace the existing one, or leave empty to remove the lock.' : 'Leave empty to remove the lock.'}</p>
          ${passwordInputHtml({ id: 'lock-password', placeholder: 'File password', value: state.lockPassword })}
        </div>
        <div class="modal-actions">
          <button class="btn btn-ghost" id="cancel-lock">Cancel</button>
          <button class="btn btn-primary" id="confirm-lock">Save</button>
        </div>
      </div>
    `;
    bindPasswordToggles(el);
    const pwInput = el.querySelector('#lock-password');
    pwInput.addEventListener('input', (e) => { state.lockPassword = e.target.value; });
    pwInput.addEventListener('keydown', (e) => { if (e.key === 'Enter') saveLockPassword(); });
    el.querySelector('#close-lock-modal').addEventListener('click', closeLockModal);
    el.querySelector('#cancel-lock').addEventListener('click', closeLockModal);
    el.querySelector('#confirm-lock').addEventListener('click', saveLockPassword);
    el.addEventListener('click', (e) => { if (e.target === el) closeLockModal(); });
  }

  function renderUnlockModal() {
    const el = container.querySelector('#unlock-modal');
    if (!el) return;
    if (!state.unlockItem) { el.classList.add('hidden'); el.innerHTML = ''; return; }
    el.classList.remove('hidden');
    const actionLabel = state.unlockAction === 'preview' ? 'Preview' : 'Download';
    el.innerHTML = `
      <div class="modal card">
        <div class="modal-head"><h3>${actionLabel} Locked File</h3><button class="action-btn modal-close" id="close-unlock-modal" aria-label="Close">${iconSvg('close', 16)}</button></div>
        <div class="modal-body">
          <p class="muted small">Enter the file password to ${state.unlockAction === 'preview' ? 'preview' : 'download'} <strong>${escapeHtml(state.unlockItem.name || state.unlockItem.item_name || '')}</strong>.</p>
          ${passwordInputHtml({ id: 'unlock-password', placeholder: 'File password', value: state.unlockPassword })}
        </div>
        <div class="modal-actions">
          <button class="btn btn-ghost" id="cancel-unlock">Cancel</button>
          <button class="btn btn-primary" id="confirm-unlock">${actionLabel}</button>
        </div>
      </div>
    `;
    bindPasswordToggles(el);
    const pwInput = el.querySelector('#unlock-password');
    pwInput.focus();
    pwInput.addEventListener('input', (e) => { state.unlockPassword = e.target.value; });
    pwInput.addEventListener('keydown', (e) => { if (e.key === 'Enter') confirmUnlock(); });
    el.querySelector('#close-unlock-modal').addEventListener('click', closeUnlockModal);
    el.querySelector('#cancel-unlock').addEventListener('click', closeUnlockModal);
    el.querySelector('#confirm-unlock').addEventListener('click', confirmUnlock);
    el.addEventListener('click', (e) => { if (e.target === el) closeUnlockModal(); });
  }

  function renderAddFriendModal() {
    const el = container.querySelector('#add-friend-modal');
    if (!el) return;
    if (!state.addFriendModal) { el.classList.add('hidden'); el.innerHTML = ''; return; }
    el.classList.remove('hidden');
    el.innerHTML = `
      <div class="modal card">
        <div class="modal-head"><h3>Add Friend</h3><button class="action-btn modal-close" id="close-add-friend" aria-label="Close">${iconSvg('close', 16)}</button></div>
        <div class="modal-body">
          <input type="email" id="add-friend-email" class="input" value="${escapeHtml(state.addFriendEmail)}" placeholder="friend@example.com" />
        </div>
        <div class="modal-actions">
          <button class="btn btn-ghost" id="cancel-add-friend">Cancel</button>
          <button class="btn btn-primary" id="confirm-add-friend">Send Request</button>
        </div>
      </div>
    `;
    const input = el.querySelector('#add-friend-email');
    input.focus();
    input.addEventListener('input', (e) => { state.addFriendEmail = e.target.value; });
    input.addEventListener('keydown', (e) => { if (e.key === 'Enter') addFriend(); });
    el.querySelector('#close-add-friend').addEventListener('click', closeAddFriend);
    el.querySelector('#cancel-add-friend').addEventListener('click', closeAddFriend);
    el.querySelector('#confirm-add-friend').addEventListener('click', addFriend);
    el.addEventListener('click', (e) => { if (e.target === el) closeAddFriend(); });
  }

  function renderBulkActionModal() {
    const el = container.querySelector('#bulk-action-modal');
    if (!el) return;
    if (!state.bulkActionModal) { el.classList.add('hidden'); el.innerHTML = ''; return; }
    el.classList.remove('hidden');
    const mode = state.bulkActionModal;
    if (mode === 'share') {
      const friendOptions = (state.friends ?? []).map((f) => `
        <label class="share-friend-row">
          <input type="checkbox" value="${escapeHtml(f.friend_email)}" ${state.bulkShareRecipients.has(f.friend_email) ? 'checked' : ''} />
          <span>${escapeHtml(f.friend_name)} <span class="muted">(${escapeHtml(f.friend_email)})</span></span>
        </label>
      `).join('');
      el.innerHTML = `
        <div class="modal card">
          <div class="modal-head"><h3>Share Selected Files</h3><button class="action-btn modal-close" id="close-bulk-modal" aria-label="Close">${iconSvg('close', 16)}</button></div>
          <div class="modal-body">
            <div class="share-friend-list">${(state.friends ?? []).length > 0 ? friendOptions : '<p class="muted small">No friends yet.</p>'}</div>
          </div>
          <div class="modal-actions">
            <button class="btn btn-ghost" id="cancel-bulk">Cancel</button>
            <button class="btn btn-primary" id="confirm-bulk">Share</button>
          </div>
        </div>
      `;
      el.querySelectorAll('.share-friend-row input').forEach((cb) => {
        cb.addEventListener('change', (e) => {
          if (e.target.checked) state.bulkShareRecipients.add(e.target.value);
          else state.bulkShareRecipients.delete(e.target.value);
          update();
        });
      });
      el.querySelector('#confirm-bulk').addEventListener('click', saveBulkShare);
    } else {
      el.innerHTML = `
        <div class="modal card">
          <div class="modal-head"><h3>Set Password</h3><button class="action-btn modal-close" id="close-bulk-modal" aria-label="Close">${iconSvg('close', 16)}</button></div>
          <div class="modal-body">
            ${passwordInputHtml({ id: 'bulk-password', placeholder: 'Leave empty to remove', value: state.bulkPassword })}
          </div>
          <div class="modal-actions">
            <button class="btn btn-ghost" id="cancel-bulk">Cancel</button>
            <button class="btn btn-primary" id="confirm-bulk">Save</button>
          </div>
        </div>
      `;
      bindPasswordToggles(el);
      const pwInput = el.querySelector('#bulk-password');
      pwInput.addEventListener('input', (e) => { state.bulkPassword = e.target.value; });
      el.querySelector('#confirm-bulk').addEventListener('click', saveBulkPassword);
    }
    el.querySelector('#close-bulk-modal').addEventListener('click', closeBulkActionModal);
    el.querySelector('#cancel-bulk').addEventListener('click', closeBulkActionModal);
    el.addEventListener('click', (e) => { if (e.target === el) closeBulkActionModal(); });
  }

  let alertTimer = null;
  function renderAlert() {
    const el = container.querySelector('#alert-container');
    if (!el) return;
    if (!state.error) { el.innerHTML = ''; return; }
    el.innerHTML = `<div class="alert-toast alert-error"><span>${escapeHtml(state.error)}</span></div>`;
    if (alertTimer) clearTimeout(alertTimer);
    alertTimer = setTimeout(() => { state.error = ''; update(); }, 3000);
  }

  let toastTimer = null;
  function renderToasts() {
    const el = container.querySelector('#toast-container');
    if (!el) return;
    const active = state.uploads.filter((u) => !u.done && !u.error);
    if (active.length === 0) { el.innerHTML = ''; return; }
    el.innerHTML = active.map((u) => `
      <div class="toast-card upload-toast">
        <div class="toast-pct">${u.progress}%</div>
        <div class="toast-progress"><div class="toast-bar" style="width:${u.progress}%"></div></div>
      </div>
    `).join('');
  }

  function showSuccess(message) {
    const el = container.querySelector('#alert-container');
    if (!el) return;
    el.innerHTML = `<div class="alert-toast alert-success"><span>${escapeHtml(message)}</span></div>`;
    if (alertTimer) clearTimeout(alertTimer);
    alertTimer = setTimeout(() => { state.error = ''; update(); }, 3000);
  }

  function showError(message) {
    state.error = message;
    update();
  }

  function emptyStateHtml(title = 'Nothing here yet', message = 'Upload a file or create a folder to get started.') {
    return `<div class="empty card"><div class="empty-icon">${iconSvg('box', 48)}</div><h3>${escapeHtml(title)}</h3><p class="muted">${escapeHtml(message)}</p></div>`;
  }

  function fmtSize(bytes) {
    if (!bytes) return '—';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let i = 0;
    let size = bytes;
    while (size >= 1024 && i < units.length - 1) { size /= 1024; i++; }
    return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
  }

  function fmtDate(d) {
    if (!d) return '—';
    return new Date(d).toLocaleDateString();
  }

  mount();
}
