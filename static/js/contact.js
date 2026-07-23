(function () {
  const trigger = document.getElementById('contact-trigger');
  const overlay = document.getElementById('contact-overlay');
  const closeBtn = document.getElementById('contact-close');
  const form = document.getElementById('contact-form');
  const successEl = document.getElementById('contact-success');
  const errorEl = document.getElementById('contact-error');

  if (!overlay) return;

  function openModal() {
    overlay.classList.add('is-open');
    overlay.setAttribute('aria-hidden', 'false');
    document.body.style.overflow = 'hidden';
    const first = overlay.querySelector('input, textarea, button');
    if (first) first.focus();
  }

  function closeModal() {
    overlay.classList.remove('is-open');
    overlay.setAttribute('aria-hidden', 'true');
    document.body.style.overflow = '';
  }

  if (trigger) trigger.addEventListener('click', function (e) { e.preventDefault(); openModal(); });
  if (closeBtn) closeBtn.addEventListener('click', closeModal);

  overlay.addEventListener('click', function (e) {
    if (e.target === overlay) closeModal();
  });

  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape' && overlay.classList.contains('is-open')) closeModal();
  });

  if (form) {
    form.addEventListener('submit', async function (e) {
      e.preventDefault();
      const btn = form.querySelector('.contact-submit');
      const formspree = form.dataset.formspree;
      const email = form.dataset.email;

      btn.textContent = 'Sending...';
      btn.disabled = true;

      if (formspree) {
        try {
          const res = await fetch(formspree, {
            method: 'POST',
            body: new FormData(form),
            headers: { 'Accept': 'application/json' }
          });
          if (res.ok) {
            form.hidden = true;
            successEl.hidden = false;
          } else {
            errorEl.hidden = false;
            btn.textContent = 'Send message';
            btn.disabled = false;
          }
        } catch (_) {
          errorEl.hidden = false;
          btn.textContent = 'Send message';
          btn.disabled = false;
        }
      } else {
        const name = document.getElementById('contact-name').value;
        const msg = document.getElementById('contact-message').value;
        const subject = encodeURIComponent('Message from DeployCraft.io');
        const body = encodeURIComponent('Name: ' + name + '\n\n' + msg);
        window.location.href = 'mailto:' + email + '?subject=' + subject + '&body=' + body;
        btn.textContent = 'Send message';
        btn.disabled = false;
      }
    });
  }
})();
