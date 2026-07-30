import { iconSvg } from './icons.js';

export function passwordInputHtml({ id, label, placeholder, value = '', name, required = false, className = 'input' }) {
  const inputName = name || id;
  const labelHtml = label ? `<label class="input-label" for="${id}">${label}</label>` : '';
  return `
    <div class="password-field">
      ${labelHtml}
      <div class="password-wrap">
        <input class="${className}" type="password" id="${id}" name="${inputName}" placeholder="${placeholder || ''}" value="${value}" autocomplete="new-password" ${required ? 'required' : ''} />
        <button type="button" class="password-toggle" data-toggle="${id}" title="Show password">${iconSvg('eye', 18)}</button>
      </div>
    </div>
  `;
}

export function bindPasswordToggles(container) {
  container.querySelectorAll('.password-toggle').forEach((btn) => {
    const inputId = btn.dataset.toggle;
    const input = container.querySelector(`#${inputId}`);
    if (!input) return;
    btn.addEventListener('click', () => {
      const isHidden = input.type === 'password';
      input.type = isHidden ? 'text' : 'password';
      btn.innerHTML = iconSvg(isHidden ? 'eyeOff' : 'eye', 18);
      btn.title = isHidden ? 'Hide password' : 'Show password';
    });
  });
}
