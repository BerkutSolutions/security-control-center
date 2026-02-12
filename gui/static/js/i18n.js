(() => {
  const defaultLang = localStorage.getItem('berkut_lang') || 'ru';
  let strings = {};
  let currentLang = defaultLang;
  let loading;

  async function load(lang) {
    if (loading && lang === currentLang) {
      return loading;
    }
    currentLang = lang;
    loading = (async () => {
      const res = await fetch(`/static/i18n/${lang}.json`);
      strings = await res.json();
      localStorage.setItem('berkut_lang', lang);
      apply();
      const switcher = document.getElementById('language-switcher');
      if (switcher) switcher.value = lang;
    })();
    return loading;
  }

  function t(key) {
    return strings[key] || key;
  }

  function apply() {
    document.querySelectorAll('[data-i18n]').forEach(el => {
      const key = el.getAttribute('data-i18n');
      el.textContent = t(key);
    });
    document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
      const key = el.getAttribute('data-i18n-placeholder');
      el.setAttribute('placeholder', t(key));
    });
  }

  window.BerkutI18n = { load, t, apply, currentLang: () => currentLang };
  load(defaultLang).catch(console.error);
})();
