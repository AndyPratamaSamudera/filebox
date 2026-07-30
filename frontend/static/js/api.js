import { auth } from './auth.js';

const BASE = '/api/v1';

async function refreshAccess() {
  const refresh = auth.refresh;
  if (!refresh) throw new Error('no session');
  const res = await fetch(`${BASE}/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refresh }),
  });
  if (!res.ok) throw new Error('session expired');
  const json = await res.json();
  if (!json.success) throw new Error(json.message);
  const { access_token, refresh_token } = json.data.tokens;
  auth.apply(access_token, refresh_token, json.data.user);
  return access_token;
}

async function authFetch(url, options = {}) {
  const headers = { ...(options.headers || {}) };
  const access = auth.access;
  if (access) headers.Authorization = `Bearer ${access}`;
  let res = await fetch(url, { ...options, headers });
  if (res.status === 401 && access) {
    const newAccess = await refreshAccess();
    headers.Authorization = `Bearer ${newAccess}`;
    res = await fetch(url, { ...options, headers });
  }
  return res;
}

export async function api(path, { method = 'GET', body, query, raw = false } = {}) {
  let url = `${BASE}${path}`;
  if (query) {
    const params = new URLSearchParams(
      Object.fromEntries(Object.entries(query).filter(([, v]) => v !== undefined && v !== null))
    );
    const qs = params.toString();
    if (qs) url += `?${qs}`;
  }
  const headers = {};
  if (body) headers['Content-Type'] = 'application/json';
  const res = await authFetch(url, { method, headers, body: body ? JSON.stringify(body) : undefined });
  if (raw) return res;
  const json = await res.json().catch(() => ({}));
  if (!res.ok || !json.success) throw new Error(json.message || `request failed (${res.status})`);
  return json.data;
}

export async function uploadItem(file, directory, meta = {}) {
  const form = new FormData();
  form.append('file', file);
  if (directory) form.append('directory', directory);
  Object.entries(meta).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== '') form.append(k, String(v));
  });
  const res = await authFetch(`${BASE}/item/upload`, { method: 'POST', body: form });
  const json = await res.json().catch(() => ({}));
  if (!res.ok || !json.success) throw new Error(json.message || 'upload failed');
  return json.data;
}

export function itemUrl(directory, kind, password = '') {
  const params = new URLSearchParams();
  if (auth.access) params.set('token', auth.access);
  if (directory) params.set('directory', directory);
  if (password) params.set('password', password);
  const qs = params.toString();
  return `${BASE}/item/${kind}${qs ? '?' + qs : ''}`;
}

export async function getUploadConfig() {
  return api('/config/upload');
}

export async function createChunkSession(meta) {
  return api('/upload/chunk/init', { method: 'POST', body: meta });
}

export async function uploadChunk(sessionId, index, chunkBlob) {
  const form = new FormData();
  form.append('index', String(index));
  form.append('chunk', chunkBlob);
  const res = await authFetch(`${BASE}/upload/chunk/${encodeURIComponent(sessionId)}`, { method: 'POST', body: form });
  const json = await res.json().catch(() => ({}));
  if (!res.ok || !json.success) throw new Error(json.message || 'chunk upload failed');
  return json.data;
}

export async function completeChunkSession(sessionId) {
  return api(`/upload/chunk/${encodeURIComponent(sessionId)}/complete`, { method: 'POST' });
}

export async function search(q) {
  return api('/item/search', { query: { q } });
}

export async function listItems(directory = '') {
  return api('/item/list', { query: { directory } }) ?? [];
}

export async function getItemDetail(directory) {
  return api('/item/detail', { query: { directory } });
}

export async function createFolder(directory) {
  return api('/item/folder', { method: 'POST', body: { directory } });
}

export async function updateItem(directory, body) {
  return api('/item/update', { method: 'PUT', body: { directory, ...body } });
}

export async function deleteItem(directory) {
  return api('/item/delete', { method: 'DELETE', query: { directory } });
}

export async function listSharedItems() {
  return api('/item/shared') ?? [];
}

export async function listFavorites() {
  return api('/item/favorites') ?? [];
}

export async function listApiKeys() {
  return api('/api-keys') ?? [];
}

export async function createApiKey(body) {
  return api('/api-keys', { method: 'POST', body });
}

export async function revokeApiKey(id) {
  return api(`/api-keys/${id}`, { method: 'DELETE' });
}

export async function revealApiKey(id, password) {
  return api(`/api-keys/${id}/reveal`, { method: 'POST', body: { password } });
}
