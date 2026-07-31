import { auth } from './auth.js?v=14';
import { renderLogin } from './pages/login.js?v=14';
import { renderRegister } from './pages/register.js?v=14';
import { renderDashboard } from './pages/dashboard.js?v=14';

const app = document.getElementById('app');

function route(path) {
  const clean = path.replace(/\/$/, '') || '/';
  if (!auth.access && clean !== '/register') return renderLoginPage();
  if (clean === '/register') return renderRegisterPage();
  if (clean === '/login') return renderLoginPage();
  return renderDashboardPage();
}

function renderLoginPage() {
  renderLogin(app, () => navigate('/'));
}

function renderRegisterPage() {
  renderRegister(app, () => navigate('/'));
}

function renderDashboardPage() {
  renderDashboard(app, () => navigate('/login'));
}

function navigate(path) {
  history.pushState(null, '', path);
  route(path);
}

window.addEventListener('popstate', () => route(location.pathname));

document.addEventListener('click', (e) => {
  const link = e.target.closest('a[data-link]');
  if (!link) return;
  e.preventDefault();
  navigate(link.getAttribute('href'));
});

route(location.pathname);
