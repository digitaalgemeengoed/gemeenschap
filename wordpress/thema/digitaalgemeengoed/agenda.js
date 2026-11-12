/* De agenda (wordpress/dg-agenda.php): het formulier (van ons of elders), het
   filter op werkgroep of project, link kopiëren en bevestigen. Zonder
   JavaScript werkt alles ook, alleen met wat meer klikken. */
(function () {
  'use strict';

  // Het formulier: alleen de velden voor "van ons" of "elders".
  document.querySelectorAll('[data-agenda]').forEach(function (f) {
    var zet = function () {
      var r = f.querySelector('input[name=soort]:checked');
      f.dataset.soort = r ? r.value : 'eigen';
      var extern = f.dataset.soort === 'extern';
      f.querySelector('input[name=extern_link]').required = extern;
      f.querySelector('select[name=tag]').required = !extern;
    };
    f.querySelectorAll('input[name=soort]').forEach(function (r) { r.addEventListener('change', zet); });
    zet();
    f.addEventListener('submit', function () {
      var knop = f.querySelector('button[type=submit]');
      knop.disabled = true;
      knop.textContent = 'Bezig…';
    });
  });

  // Werkgroep of project kiezen: meteen tonen.
  document.querySelectorAll('select[data-direct]').forEach(function (s) {
    s.addEventListener('change', function () { s.form.submit(); });
  });

  document.addEventListener('click', function (e) {
    var k = e.target.closest('[data-kopieer]');
    if (!k) { return; }
    navigator.clipboard.writeText(k.dataset.kopieer).then(function () {
      var m = document.createElement('div');
      m.className = 'dg-ber-gekopieerd';
      m.setAttribute('role', 'status');
      m.textContent = 'Link gekopieerd';
      document.body.appendChild(m);
      setTimeout(function () { m.remove(); }, 2200);
    });
  });

  document.querySelectorAll('form[data-bevestig]').forEach(function (f) {
    f.addEventListener('submit', function (e) {
      if (!window.confirm(f.dataset.bevestig)) { e.preventDefault(); }
    });
  });
})();
