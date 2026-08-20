import { iconSvg } from '../icons.js';

export function renderLanding(container, onLogin, onRegister) {
  container.innerHTML = `
    <div class="landing-page">
      <header class="landing-nav">
        <a class="landing-brand" href="/" data-link>
          <span class="landing-logo">${iconSvg('box', 24)}</span>
          <span>File<span>Box</span></span>
        </a>
        <nav class="landing-links" aria-label="Main navigation">
          <a href="#features">Fitur</a>
          <a href="#security">Keamanan</a>
          <button class="btn btn-ghost" id="landing-login">Masuk</button>
          <button class="btn btn-primary" id="landing-register">Buat akun</button>
        </nav>
      </header>

      <main>
        <section class="landing-hero">
          <div class="hero-copy">
            <div class="eyebrow"><span></span> Private cloud storage, made simple</div>
            <h1>File Anda.<br /><em>Ruang Anda.</em></h1>
            <p class="hero-lead">Simpan, kelola, dan bagikan file dengan tenang. FileBox menghadirkan cloud pribadi yang ringan, aman, dan berjalan di perangkat Anda sendiri.</p>
            <div class="hero-actions">
              <button class="btn btn-primary btn-large" id="hero-register">Mulai sekarang →</button>
              <button class="text-link" id="hero-login">Saya sudah punya akun <span>→</span></button>
            </div>
            <div class="hero-note">Gratis untuk memulai <span>•</span> Tanpa kartu kredit</div>
          </div>
          <div class="hero-visual" aria-label="FileBox dashboard preview">
            <div class="glow"></div>
            <div class="preview-window">
              <div class="preview-top"><div class="window-dots"><i></i><i></i><i></i></div><span>FileBox / My files</span><span class="preview-menu">•••</span></div>
              <div class="preview-body">
                <aside><div class="mini-brand">${iconSvg('box', 16)} FileBox</div><div class="mini-nav active">${iconSvg('home', 14)} Beranda</div><div class="mini-nav">${iconSvg('star', 14)} Favorit</div><div class="mini-nav">${iconSvg('users', 14)} Dibagikan</div><div class="mini-storage"><b>Storage</b><div><span></span></div><small>48.2 GB tersedia</small></div></aside>
                <div class="preview-content"><div class="preview-heading"><div><small>Selamat datang kembali</small><h3>File saya</h3></div><div class="mini-upload">+ Upload</div></div><div class="mini-stats"><div><small>Total files</small><strong>1,284</strong></div><div><small>Used space</small><strong>32.8 GB</strong></div></div><div class="recent-head"><b>File terbaru</b><small>Lihat semua →</small></div><div class="mini-files"><div>${iconSvg('folder', 20)}<span>Projects</span><small>Folder</small></div><div>${iconSvg('image', 20)}<span>mountain.jpg</span><small>2.4 MB</small></div><div>${iconSvg('file', 20)}<span>notes.pdf</span><small>840 KB</small></div></div></div>
              </div>
            </div>
          </div>
        </section>

        <section class="trust-strip"><span>Dirancang untuk perangkat Anda</span><span class="trust-line"></span><span>Ringan di resource</span><span class="trust-line"></span><span>Privasi sebagai standar</span></section>

        <section class="feature-section" id="features"><div class="section-intro"><div class="eyebrow"><span></span> Semua yang Anda butuhkan</div><h2>Cloud pribadi yang terasa<br /><em>sederhana.</em></h2><p>Fokus pada file Anda, bukan konfigurasi yang rumit.</p></div><div class="feature-grid"><article><div class="feature-icon">${iconSvg('lock', 21)}</div><h3>Aman secara default</h3><p>Enkripsi AES-256 menjaga data Anda tetap privat, bahkan saat tersimpan di disk.</p></article><article><div class="feature-icon">${iconSvg('upload', 21)}</div><h3>Upload tanpa batas ribet</h3><p>Upload langsung atau gunakan chunk upload untuk file berukuran besar.</p></article><article><div class="feature-icon">${iconSvg('share', 21)}</div><h3>Bagikan dengan kendali</h3><p>Berbagi file dengan teman tetap aman karena Anda yang menentukan aksesnya.</p></article></div></section>
        <section class="landing-cta" id="security"><div><div class="eyebrow"><span></span> Data tetap milik Anda</div><h2>Mulai membangun ruang<br />digital Anda sendiri.</h2></div><button class="btn btn-primary btn-large" id="bottom-register">Buat akun gratis →</button></section>
      </main>
      <footer class="landing-footer"><span class="landing-brand">${iconSvg('box', 18)} FileBox</span><span>Private storage. Simple by design.</span><span>© 2026 FileBox</span></footer>
    </div>
  `;
  container.querySelectorAll('#landing-login, #hero-login').forEach((el) => el.addEventListener('click', onLogin));
  container.querySelectorAll('#landing-register, #hero-register, #bottom-register').forEach((el) => el.addEventListener('click', onRegister));
}