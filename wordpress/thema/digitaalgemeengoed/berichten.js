/* Nieuws en berichten (wordpress/dg-berichten.php): meer lezen, reageren,
   delen, reposten, verzenden, melden, opmerkingen en het opstelformulier.
   Zonder JavaScript werkt plaatsen, bewerken, verwijderen en reageren in
   opmerkingen gewoon; reacties, verzenden en melden hebben JavaScript nodig. */
(function () {
  'use strict';
  var cfg = window.dgBerichten || {};

  function api(pad, methode, data) {
    return fetch(cfg.rest + pad, {
      method: methode || 'GET',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'X-WP-Nonce': cfg.nonce },
      body: data ? JSON.stringify(data) : undefined
    }).then(function (r) {
      return r.json().then(function (j) {
        if (!r.ok) { throw new Error(j && j.message ? j.message : 'Dat lukte niet.'); }
        return j;
      });
    });
  }

  function melding(tekst) {
    var m = document.createElement('div');
    m.className = 'dg-ber-gekopieerd';
    m.setAttribute('role', 'status');
    m.textContent = tekst;
    document.body.appendChild(m);
    setTimeout(function () { m.remove(); }, 2200);
  }

  // Meer lezen: alleen als de tekst echt is afgekapt.
  function meerKnoppen() {
    document.querySelectorAll('.dg-ber-tekst.is-ingeklapt').forEach(function (t) {
      var knop = t.nextElementSibling;
      if (knop && knop.classList.contains('dg-ber-meer')) {
        knop.hidden = t.scrollHeight <= t.clientHeight + 2;
      }
    });
  }
  meerKnoppen();
  window.addEventListener('load', meerKnoppen);

  // Het opstelformulier: open bij focus, flits-datum, gekozen bestanden.
  document.querySelectorAll('[data-opstellen]').forEach(function (f) {
    var tekst = f.querySelector('textarea[name=tekst]');
    var open = function () {
      f.classList.add('is-open');
      if (tekst.rows < 4) { tekst.rows = 4; }
    };
    tekst.addEventListener('focus', open);
    var flits = f.querySelector('input[name=flits]');
    var tot = f.querySelector('.dg-ber-flitstot');
    var titel = f.querySelector('input[name=titel]');
    if (flits) {
      flits.addEventListener('change', function () {
        tot.hidden = !flits.checked;
        titel.required = flits.checked;
        if (flits.checked && !titel.value) { titel.focus(); }
      });
      titel.required = flits.checked;
    }
    var lijst = f.querySelector('[data-bestanden]');
    f.querySelectorAll('input[type=file]').forEach(function (inp) {
      inp.addEventListener('change', function () {
        open();
        var namen = [];
        f.querySelectorAll('input[type=file]').forEach(function (i) {
          Array.prototype.forEach.call(i.files, function (b) {
            namen.push(b.name + ' (' + Math.max(1, Math.round(b.size / 1048576)) + ' MB)');
          });
        });
        lijst.innerHTML = '';
        namen.forEach(function (n) {
          var s = document.createElement('span');
          s.textContent = n;
          lijst.appendChild(s);
        });
      });
    });
    f.addEventListener('submit', function () {
      var knop = f.querySelector('button[type=submit]');
      knop.disabled = true;
      knop.textContent = 'Bezig…';
    });
  });

  document.addEventListener('click', function (e) {
    var el = e.target.closest('button, a');
    if (!el) { return; }

    // Meer lezen.
    if (el.classList.contains('dg-ber-meer')) {
      el.previousElementSibling.classList.remove('is-ingeklapt');
      el.remove();
      return;
    }

    // Een video pas laden na een klik.
    if (el.classList.contains('dg-ber-speel')) {
      var tpl = el.nextElementSibling;
      el.parentNode.innerHTML = tpl.innerHTML;
      return;
    }

    // Reageren: de hoofdknop (leuk, of je huidige reactie weer weg) of een keuze.
    if (el.hasAttribute('data-reactie')) {
      var kaart = el.closest('[data-bericht]');
      var blok = el.closest('[data-reageer]');
      blok.classList.remove('is-open');
      api('berichten/' + kaart.dataset.bericht + '/reactie', 'POST', { soort: el.dataset.reactie })
        .then(function (j) {
          kaart.querySelector('[data-telling]').outerHTML = j.telling;
          blok.outerHTML = j.knop;
        })
        .catch(function (f) { melding(f.message); });
      return;
    }
    if (el.classList.contains('dg-ber-meer-reacties')) {
      var b = el.closest('[data-reageer]');
      var open = b.classList.toggle('is-open');
      el.setAttribute('aria-expanded', open ? 'true' : 'false');
      if (open) { b.querySelector('.dg-ber-keuze button').focus(); }
      return;
    }

    // Link kopiëren.
    if (el.hasAttribute('data-kopieer')) {
      navigator.clipboard.writeText(el.dataset.kopieer).then(function () { melding('Link gekopieerd'); });
      el.closest('details').removeAttribute('open');
      return;
    }

    // Reposten.
    if (el.hasAttribute('data-repost')) {
      var d = document.getElementById('dg-ber-repost');
      var f = d.querySelector('form');
      f.reset();
      f.repost_van.value = el.dataset.repost;
      var leden = el.dataset.openbaar !== '1';
      f.zichtbaar.value = leden ? 'leden' : 'openbaar';
      f.zichtbaar.disabled = leden;
      d.querySelector('[data-alleen-leden]').hidden = !leden;
      d.showModal();
      return;
    }

    // Verzenden als privébericht.
    if (el.hasAttribute('data-sturen')) {
      var ds = document.getElementById('dg-ber-sturen');
      ds.dataset.bericht = el.dataset.sturen;
      ds.querySelector('form').reset();
      ds.querySelector('[data-ontvangers]').innerHTML = '';
      ds.querySelector('[data-uitslag]').textContent = '';
      ds.showModal();
      ds.querySelector('[data-zoek-lid]').focus();
      return;
    }

    // Melden.
    if (el.hasAttribute('data-melden')) {
      var dm = document.getElementById('dg-ber-melden');
      dm.dataset.bericht = el.dataset.melden;
      dm.querySelector('form').reset();
      dm.querySelector('[data-uitslag]').textContent = '';
      el.closest('details').removeAttribute('open');
      dm.showModal();
      return;
    }

    if (el.hasAttribute('data-sluit')) {
      el.closest('dialog').close();
      return;
    }

    // Antwoorden op een opmerking.
    if (el.hasAttribute('data-antwoord')) {
      var form = document.querySelector('.dg-ber-opmerkingform');
      form.comment_parent.value = el.dataset.antwoord;
      var op = form.querySelector('.dg-ber-antwoord-op');
      op.hidden = false;
      op.querySelector('strong').textContent = el.dataset.naam;
      form.comment.focus();
      return;
    }
    if (el.hasAttribute('data-geen-antwoord')) {
      var fo = el.closest('form');
      fo.comment_parent.value = '0';
      fo.querySelector('.dg-ber-antwoord-op').hidden = true;
      return;
    }
    if (el.hasAttribute('data-naar-opmerking')) {
      var veld = document.getElementById('dg-ber-opmerking');
      if (veld) { e.preventDefault(); veld.focus(); veld.scrollIntoView({ block: 'center' }); }
    }
  });

  // Klik ernaast: open menu's en de reactiekeuze dicht.
  document.addEventListener('click', function (e) {
    document.querySelectorAll('.dg-ber-menu[open]').forEach(function (d) {
      if (!d.contains(e.target)) { d.removeAttribute('open'); }
    });
    document.querySelectorAll('[data-reageer].is-open').forEach(function (b) {
      if (!b.contains(e.target)) { b.classList.remove('is-open'); }
    });
  });
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') {
      document.querySelectorAll('.dg-ber-menu[open]').forEach(function (d) { d.removeAttribute('open'); });
      document.querySelectorAll('[data-reageer].is-open').forEach(function (b) { b.classList.remove('is-open'); });
    }
  });

  // Bevestigen voor verwijderen en verbergen.
  document.querySelectorAll('form[data-bevestig]').forEach(function (f) {
    f.addEventListener('submit', function (e) {
      if (!window.confirm(f.dataset.bevestig)) { e.preventDefault(); }
    });
  });

  // Reposten: een uitgeschakeld veld gaat niet mee; dan staat "leden" vast.
  var repost = document.getElementById('dg-ber-repost');
  if (repost) {
    repost.querySelector('form').addEventListener('submit', function () {
      this.zichtbaar.disabled = false;
    });
  }

  // Verzenden: leden zoeken en kiezen.
  var sturen = document.getElementById('dg-ber-sturen');
  if (sturen) {
    var zoek = sturen.querySelector('[data-zoek-lid]');
    var lijst = document.getElementById('dg-ber-ledenlijst');
    var ontv = sturen.querySelector('[data-ontvangers]');
    var gevonden = {};
    var wacht;
    var kies = function () {
      var id = gevonden[zoek.value];
      if (!id || ontv.querySelector('[data-id="' + id + '"]') || ontv.children.length >= 10) { return; }
      var li = document.createElement('li');
      li.dataset.id = id;
      li.textContent = zoek.value;
      var x = document.createElement('button');
      x.type = 'button';
      x.setAttribute('aria-label', 'Weghalen');
      x.textContent = '×';
      x.addEventListener('click', function () { li.remove(); });
      li.appendChild(x);
      ontv.appendChild(li);
      zoek.value = '';
    };
    zoek.addEventListener('input', function () {
      if (gevonden[zoek.value]) { kies(); return; }
      clearTimeout(wacht);
      if (zoek.value.length < 2) { return; }
      wacht = setTimeout(function () {
        api('leden?zoek=' + encodeURIComponent(zoek.value)).then(function (rij) {
          lijst.innerHTML = '';
          rij.forEach(function (l) {
            gevonden[l.naam] = l.id;
            var o = document.createElement('option');
            o.value = l.naam;
            lijst.appendChild(o);
          });
        });
      }, 250);
    });
    zoek.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') { e.preventDefault(); kies(); }
    });
    sturen.querySelector('form').addEventListener('submit', function (e) {
      e.preventDefault();
      var uit = sturen.querySelector('[data-uitslag]');
      kies();
      var aan = Array.prototype.map.call(ontv.children, function (li) { return parseInt(li.dataset.id, 10); });
      if (!aan.length) { uit.className = 'dg-ber-uitslag is-fout'; uit.textContent = 'Kies eerst een lid.'; return; }
      api('berichten/' + sturen.dataset.bericht + '/sturen', 'POST', { aan: aan, tekst: this.tekst.value })
        .then(function (j) { uit.className = 'dg-ber-uitslag'; uit.textContent = j.tekst; ontv.innerHTML = ''; })
        .catch(function (f) { uit.className = 'dg-ber-uitslag is-fout'; uit.textContent = f.message; });
    });
  }

  var melden = document.getElementById('dg-ber-melden');
  if (melden) {
    melden.querySelector('form').addEventListener('submit', function (e) {
      e.preventDefault();
      var uit = melden.querySelector('[data-uitslag]');
      api('berichten/' + melden.dataset.bericht + '/melden', 'POST', { reden: this.reden.value })
        .then(function (j) { uit.className = 'dg-ber-uitslag'; uit.textContent = j.tekst; })
        .catch(function (f) { uit.className = 'dg-ber-uitslag is-fout'; uit.textContent = f.message; });
    });
  }
})();
