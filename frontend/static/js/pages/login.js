import { auth } from '../auth.js';
import { iconSvg } from '../icons.js';
import { passwordInputHtml, bindPasswordToggles } from '../ui.js';

export function renderLogin(container, onLogin) {
  container.innerHTML = `
    <div class="auth-page">
      <div class="auth-card">
        <div class="auth-brand">
          <span class="brand-logo">${iconSvg('box', 28)}</span>
          <h1>FileBox</h1>
        </div>
        <form id="login-form" class="auth-form">
          <input class="input" type="email" name="email" placeholder="Email" required />
          ${passwordInputHtml({ id: 'login-password', name: 'password', placeholder: 'Password', required: true })}
          <button class="btn btn-primary btn-block" type="submit">Login</button>
        </form>
        <p class="auth-footer">Don't have an account? <a href="/register" data-link>Register</a></p>
        <div id="login-error" class="alert alert-error hidden"></div>
      </div>
    </div>
  `;

  const form = container.querySelector('#login-form');
  const errorEl = container.querySelector('#login-error');
  bindPasswordToggles(container);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    errorEl.classList.add('hidden');
    const email = form.email.value.trim();
    const password = form.password.value;

    try {
      const res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });
      const json = await res.json();
      if (!res.ok || !json.success) throw new Error(json.message || 'login failed');
      const { access_token, refresh_token } = json.data.tokens;
      auth.apply(access_token, refresh_token, json.data.user);
      if (json.data.api_key) {
        showApiKeyModal(container, json.data.api_key, onLogin);
      } else {
        onLogin();
      }
    } catch (err) {
      errorEl.textContent = err.message;
      errorEl.classList.remove('hidden');
    }
  });
}

function showApiKeyModal(container, apiKey, onClose) {
  const modal = document.createElement('div');
  modal.className = 'modal-overlay';
  modal.innerHTML = `
    <div class="modal card api-key-modal">
      <h3>Your API Key</h3>
      <p class="muted">A default API key has been generated for you. Copy it now — it won't be shown again.</p>
      <div class="api-key-box">
        <input class="input" type="text" id="api-key-value" value="${apiKey}" readonly />
        <button class="btn btn-sm btn-ghost" id="copy-api-key">Copy</button>
      </div>
      <div class="modal-actions">
        <button class="btn btn-primary" id="close-api-key">Continue</button>
      </div>
    </div>
  `;
  container.appendChild(modal);
  const input = modal.querySelector('#api-key-value');
  modal.querySelector('#copy-api-key').addEventListener('click', () => {
    input.select();
    navigator.clipboard.writeText(input.value).catch(() => {});
  });
  modal.querySelector('#close-api-key').addEventListener('click', () => {
    modal.remove();
    onClose();
  });
}
