import { auth } from '../auth.js';
import { iconSvg } from '../icons.js';
import { passwordInputHtml, bindPasswordToggles } from '../ui.js';

export function renderRegister(container, onRegistered) {
  container.innerHTML = `
    <div class="auth-page">
      <div class="auth-card">
        <div class="auth-brand">
          <span class="brand-logo">${iconSvg('box', 28)}</span>
          <h1>FileBox</h1>
        </div>
        <form id="register-form" class="auth-form">
          <input class="input" type="email" name="email" placeholder="Email" required />
          <input class="input" type="text" name="username" placeholder="Username" required />
          ${passwordInputHtml({ id: 'register-password', name: 'password', placeholder: 'Password', required: true })}
          <button class="btn btn-primary btn-block" type="submit">Register</button>
        </form>
        <p class="auth-footer">Already have an account? <a href="/login" data-link>Login</a></p>
        <div id="register-error" class="alert alert-error hidden"></div>
      </div>
    </div>
  `;

  const form = container.querySelector('#register-form');
  const errorEl = container.querySelector('#register-error');
  bindPasswordToggles(container);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    errorEl.classList.add('hidden');
    const email = form.email.value.trim();
    const username = form.username.value.trim();
    const password = form.password.value;

    try {
      let res = await fetch('/api/v1/auth/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, username, password }),
      });
      const regJson = await res.json();
      if (!res.ok || !regJson.success) throw new Error(regJson.message || 'register failed');

      res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });
      const loginJson = await res.json();
      if (!res.ok || !loginJson.success) throw new Error(loginJson.message || 'auto login failed');
      const { access_token, refresh_token } = loginJson.data.tokens;
      auth.apply(access_token, refresh_token, loginJson.data.user);
      onRegistered();
    } catch (err) {
      errorEl.textContent = err.message;
      errorEl.classList.remove('hidden');
    }
  });
}
