// Documenten: dialogen, slepen, uploaden met voortgang, rijen aanklikken.
// Zonder JavaScript werkt alles ook, alleen eenvoudiger.
(function () {
  // Op een telefoon staat de mappenboom eerst dicht; de lijst is belangrijker.
  var nav = document.querySelector('.nav-inklap');
  if (nav && window.matchMedia && window.matchMedia('(max-width: 760px)').matches) nav.open = false;

  var voortgang = document.querySelector('.voortgang');

  // Uploaden met voortgang; daarna naar de pagina die de server teruggeeft.
  function verstuur(form, bestanden) {
    var max = parseInt(form.dataset.max || '0', 10) * 1048576;
    for (var i = 0; i < bestanden.length; i++) {
      if (max && bestanden[i].size > max) { alert(bestanden[i].name + ' is groter dan ' + form.dataset.max + ' MB.'); return; }
    }
    var data = new FormData();
    form.querySelectorAll('input[type=hidden]').forEach(function (x) { data.append(x.name, x.value); });
    for (var j = 0; j < bestanden.length; j++) data.append('bestanden', bestanden[j]);
    var xhr = new XMLHttpRequest();
    xhr.open('POST', form.action);
    var balk = form.querySelector('progress') || voortgang;
    if (balk) { balk.hidden = false; balk.value = 0; }
    xhr.upload.onprogress = function (e) { if (balk && e.lengthComputable) balk.value = Math.round(e.loaded / e.total * 100); };
    xhr.onload = function () { window.location.href = xhr.responseURL || window.location.href; };
    xhr.onerror = function () { if (balk) balk.hidden = true; alert('De upload is niet gelukt. Probeer het opnieuw.'); };
    xhr.send(data);
  }

  document.querySelectorAll('form.opdracht-upload, form.nieuwe-versie').forEach(function (form) {
    var invoer = form.querySelector('input[type=file]');
    invoer.addEventListener('change', function () { if (invoer.files.length) verstuur(form, invoer.files); });
  });

  // Slepen op de lijst: naar de map waar je bent.
  var lijst = document.querySelector('[data-sleepdoel]');
  var uploadForm = document.querySelector('form.opdracht-upload');
  if (lijst && uploadForm) {
    var diepte = 0;
    lijst.addEventListener('dragenter', function (e) { if (e.dataTransfer && Array.prototype.indexOf.call(e.dataTransfer.types, 'Files') >= 0) { diepte++; lijst.classList.add('erboven'); } });
    lijst.addEventListener('dragover', function (e) { e.preventDefault(); });
    lijst.addEventListener('dragleave', function () { if (--diepte <= 0) { diepte = 0; lijst.classList.remove('erboven'); } });
    lijst.addEventListener('drop', function (e) {
      e.preventDefault(); diepte = 0; lijst.classList.remove('erboven');
      if (e.dataTransfer && e.dataTransfer.files.length) verstuur(uploadForm, e.dataTransfer.files);
    });
  }
  // Slepen op een vak (nieuwe versie).
  document.querySelectorAll('.vak').forEach(function (vak) {
    var form = vak.closest('form');
    ['dragenter', 'dragover'].forEach(function (n) { vak.addEventListener(n, function (e) { e.preventDefault(); e.stopPropagation(); vak.classList.add('erboven'); }); });
    ['dragleave', 'drop'].forEach(function (n) { vak.addEventListener(n, function () { vak.classList.remove('erboven'); }); });
    vak.addEventListener('drop', function (e) { e.preventDefault(); e.stopPropagation(); if (e.dataTransfer.files.length) verstuur(form, [e.dataTransfer.files[0]]); });
  });

  // Dialogen.
  document.querySelectorAll('[data-open]').forEach(function (k) {
    k.addEventListener('click', function () { var d = document.getElementById(k.dataset.open); if (d && d.showModal) d.showModal(); });
  });
  document.querySelectorAll('[data-sluit]').forEach(function (k) {
    k.addEventListener('click', function () { k.closest('dialog').close(); });
  });

  // Een rij aanklikken = openen (map) of kiezen (document).
  document.querySelectorAll('tr[data-href]').forEach(function (rij) {
    rij.addEventListener('click', function (e) { if (!e.target.closest('a, button, input, select')) window.location.href = rij.dataset.href; });
  });

  // --- Contextmenu (rechtermuisknop, de menutoets, of de knop ⋯) -------------
  var lijstpaneel = document.querySelector('.lijstpaneel[data-ruimte]');
  var menu = document.querySelector('.contextmenu');
  if (lijstpaneel && menu) contextmenu(lijstpaneel, menu);

  // Link kopiëren.
  document.querySelectorAll('[data-kopieer]').forEach(function (k) {
    k.addEventListener('click', function () {
      var veld = document.getElementById(k.dataset.kopieer);
      var klaar = function () { var t = k.textContent; k.textContent = 'Gekopieerd'; setTimeout(function () { k.textContent = t; }, 1500); };
      if (navigator.clipboard && window.isSecureContext) navigator.clipboard.writeText(veld.value).then(klaar);
      else { veld.select(); try { document.execCommand('copy'); klaar(); } catch (e) {} }
    });
  });
  // Delen via de telefoon of de computer (Web Share), als de browser dat kan.
  document.querySelectorAll('[data-webshare]').forEach(function (k) {
    if (!navigator.share) return;
    k.hidden = false;
    k.addEventListener('click', function () { navigator.share({ title: k.dataset.titel, text: k.dataset.tekst, url: k.dataset.link }).catch(function () {}); });
  });

  // Tekst kopiëren (adres, app-wachtwoord).
  document.querySelectorAll('[data-kopieer-tekst]').forEach(function (k) {
    k.addEventListener('click', function () {
      var t = document.getElementById(k.dataset.kopieerTekst).textContent.trim();
      if (navigator.clipboard && window.isSecureContext) navigator.clipboard.writeText(t).then(function () { k.textContent = 'Gekopieerd'; });
    });
  });

  // Openen in Word of LibreOffice: de tijdelijke link ophalen en het programma starten.
  document.querySelectorAll('form[data-openen]').forEach(function (form) {
    form.addEventListener('submit', function (e) {
      e.preventDefault();
      var knop = form.querySelector('button'); var tekst = knop.textContent;
      knop.disabled = true; knop.textContent = 'Openen…';
      fetch(form.action, { method: 'POST', body: new FormData(form), headers: { Accept: 'application/json' }, credentials: 'same-origin' })
        .then(function (r) { if (!r.ok) throw new Error(r.status); return r.json(); })
        .then(function (d) { window.location.href = d.adres; })
        .catch(function () { form.submit(); })
        .finally(function () { setTimeout(function () { knop.disabled = false; knop.textContent = tekst; }, 2500); });
    });
  });
  // Op de pagina "Openen in …" meteen starten.
  var meteen = document.querySelector('[data-meteen]');
  if (meteen) window.location.href = meteen.href;

  // Een recht direct opslaan bij het kiezen.
  document.querySelectorAll('select[data-direct]').forEach(function (s) {
    s.addEventListener('change', function () { s.form.submit(); });
  });

  function contextmenu(paneel, menu) {
    var ruimte = paneel.dataset.ruimte, mapID = paneel.dataset.map, hierNiveau = +paneel.dataset.niveau || 0;
    var basis = paneel.dataset.basis || location.origin;
    var NIVEAU = ['geen toegang', 'Lezen', 'Bewerken', 'Beheren'];
    var KLEMBORD = 'dg-klembord';
    var terugFocus = null;

    function klembord() { try { return JSON.parse(sessionStorage.getItem(KLEMBORD) || 'null'); } catch (e) { return null; } }
    function zetKlembord(x) { try { if (x) sessionStorage.setItem(KLEMBORD, JSON.stringify(x)); else sessionStorage.removeItem(KLEMBORD); } catch (e) {} toonKlembord(); }

    // Een melding onderin zolang er iets op het klembord staat.
    var balk = document.querySelector('.statusbalk');
    var kb = document.createElement('span'); kb.className = 'klembord'; kb.hidden = true;
    if (balk) balk.insertBefore(kb, balk.firstChild);
    function toonKlembord() {
      var k = klembord();
      kb.hidden = !k;
      if (!k) return;
      kb.textContent = '';
      kb.appendChild(document.createTextNode((k.actie === 'knippen' ? 'Geknipt: ' : 'Gekopieerd: ') + k.naam + ' '));
      if (hierNiveau >= 2) { var p = document.createElement('button'); p.type = 'button'; p.className = 'link'; p.textContent = 'Plakken hier'; p.addEventListener('click', function () { plak(mapID); }); kb.appendChild(p); kb.appendChild(document.createTextNode(' · ')); }
      var w = document.createElement('button'); w.type = 'button'; w.className = 'link'; w.textContent = 'Wissen'; w.addEventListener('click', function () { zetKlembord(null); }); kb.appendChild(w);
    }
    toonKlembord();

    function plak(doel) {
      var k = klembord(), f = document.getElementById('plakform');
      if (!k || !f) return;
      f.actie.value = k.actie; f.bron.value = k.id; f.bron_ruimte.value = k.ruimte; f.doel.value = doel || 0;
      if (k.actie === 'knippen') zetKlembord(null);
      f.submit();
    }
    function kopieerTekst(t, klaar) {
      if (navigator.clipboard && window.isSecureContext) navigator.clipboard.writeText(t).then(klaar, function () { prompt('Kopieer de link:', t); });
      else prompt('Kopieer de link:', t);
    }
    function dialoog(id, item) {
      var d = document.getElementById(id); if (!d) return;
      var f = d.querySelector('form');
      if (item && f && f.dataset.actie) f.action = f.dataset.actie.replace('{id}', item.id);
      if (id === 'dlg-naam' && item) { f.naam.value = item.naam; var wat = d.querySelector('[data-wat]'); if (wat) wat.textContent = 'Nieuwe naam van ' + (item.soort === 'map' ? 'de map' : 'het document'); }
      if (id === 'dlg-weg' && item) { d.querySelector('[data-naam]').textContent = item.naam; d.querySelector('[data-alles]').textContent = item.soort === 'map' ? ' en alles erin' : ''; }
      d.showModal();
      if (id === 'dlg-naam') { var inv = f.naam; var punt = inv.value.lastIndexOf('.'); inv.focus(); if (item && item.soort === 'document' && punt > 0) inv.setSelectionRange(0, punt); else inv.select(); }
    }
    function openIn(item, app) {
      var f = document.getElementById('openform');
      f.action = f.dataset.actie.replace('{id}', item.id); f.app.value = app;
      f.requestSubmit ? f.requestSubmit() : f.submit();
    }

    // De opdrachten voor een rij, of (zonder rij) voor de map waar je bent.
    function opdrachten(rij) {
      var k = klembord(), uit = [];
      function knop(label, doe, nodig, opties) { uit.push({ label: label, doe: doe, nodig: nodig || 0, gevaar: opties && opties.gevaar, uitleg: opties && opties.uitleg }); }
      function streep() { if (uit.length && uit[uit.length - 1] !== '-') uit.push('-'); }
      if (!rij) {
        var hier = { id: mapID, soort: 'map', naam: paneel.dataset.mapNaam };
        knop('Nieuwe map', function () { dialoog('dlg-map'); }, 2);
        knop('Uploaden…', function () { var i = document.querySelector('form.opdracht-upload input[type=file]'); if (i) i.click(); }, 2);
        if (k) knop('Plakken: ' + k.naam, function () { plak(mapID); }, 2);
        streep();
        knop('Zoeken hier', function () { location.href = '/zoeken?in=' + encodeURIComponent(ruimte) + (+mapID ? '&m=' + mapID : ''); });
        knop('Assistent: info', function () { location.href = +mapID ? '/r/' + ruimte + '/m/' + mapID + '/info' : '/r/' + ruimte + '/info'; });
        return { item: hier, niveau: hierNiveau, lijst: uit };
      }
      var item = { id: rij.dataset.id, soort: rij.dataset.soort, naam: rij.dataset.naam };
      var niveau = +rij.dataset.niveau || 0, doc = item.soort === 'document';
      var pad = '/r/' + ruimte + (doc ? '/d/' : '/m/') + item.id;
      if (doc) {
        knop('Openen', function () { location.href = pad; }, 1);
        (rij.dataset.apps || '').split('|').filter(Boolean).forEach(function (a) {
          var d = a.split(':'); knop('Openen in ' + d[1], function () { openIn(item, d[0]); }, 1);
        });
        if (rij.hasAttribute('data-bekijken')) knop('Bekijken in de browser', function () { window.open(pad + '/download?bekijken=1', '_blank', 'noopener'); }, 1);
        knop('Downloaden', function () { var a = document.createElement('a'); a.href = pad + '/download'; a.download = ''; document.body.appendChild(a); a.click(); a.remove(); }, 1);
      } else {
        knop('Openen', function () { location.href = pad; });
        if (k) knop('Plakken in deze map', function () { plak(item.id); }, 2);
      }
      streep();
      knop('Knippen', function () { zetKlembord({ actie: 'knippen', ruimte: ruimte, id: item.id, naam: item.naam, soort: item.soort }); }, 2);
      if (doc) knop('Kopiëren', function () { zetKlembord({ actie: 'kopieren', ruimte: ruimte, id: item.id, naam: item.naam, soort: item.soort }); }, 1);
      knop('Naam wijzigen…', function () { dialoog('dlg-naam', item); }, 2);
      streep();
      knop('Delen…', function () { location.href = pad + (doc ? '/delen' : '/delen'); }, 1);
      knop('Link kopiëren', function () { kopieerTekst(basis + pad, function () { meld('Link gekopieerd.'); }); }, 1);
      knop('Toegang', function () { location.href = pad + '/toegang'; }, doc ? 1 : 0);
      if (doc) knop('Versies', function () { location.href = pad + '/versies'; }, 1);
      else knop('Zoeken in deze map', function () { location.href = '/zoeken?in=' + encodeURIComponent(ruimte) + '&m=' + item.id; });
      knop('Assistent: info', function () { location.href = doc ? pad : pad + '/info'; }, doc ? 1 : 0);
      streep();
      knop('Verwijderen…', function () { dialoog('dlg-weg', item); }, 3, { gevaar: true });
      return { item: item, niveau: niveau, lijst: uit };
    }

    function meld(t) {
      var m = document.createElement('p'); m.className = 'melding zwevend'; m.setAttribute('role', 'status'); m.textContent = t;
      document.body.appendChild(m); setTimeout(function () { m.remove(); }, 2500);
    }

    function open(rij, x, y) {
      var o = opdrachten(rij);
      menu.textContent = '';
      menu.setAttribute('aria-label', 'Acties voor ' + o.item.naam);
      o.lijst.forEach(function (op) {
        if (op === '-') { var hr = document.createElement('div'); hr.className = 'scheiding'; hr.setAttribute('role', 'separator'); menu.appendChild(hr); return; }
        var b = document.createElement('button');
        b.type = 'button'; b.setAttribute('role', 'menuitem'); b.textContent = op.label; b.tabIndex = -1;
        if (op.gevaar) b.className = 'gevaar';
        if (o.niveau < op.nodig) {
          b.setAttribute('aria-disabled', 'true');
          b.title = 'Je hebt hier het recht ' + NIVEAU[o.niveau] + '; hiervoor is ' + NIVEAU[op.nodig] + ' nodig.';
        } else b.addEventListener('click', function () { sluit(false); op.doe(); });
        menu.appendChild(b);
      });
      if (menu.lastChild && menu.lastChild.className === 'scheiding') menu.lastChild.remove();
      menu.hidden = false;
      var w = menu.offsetWidth, h = menu.offsetHeight;
      menu.style.left = Math.max(8, Math.min(x, innerWidth - w - 8)) + 'px';
      menu.style.top = Math.max(8, Math.min(y, innerHeight - h - 8)) + 'px';
      document.querySelectorAll('tr.menu-open').forEach(function (r) { r.classList.remove('menu-open'); });
      if (rij) rij.classList.add('menu-open');
      var eerste = menu.querySelector('[role=menuitem]:not([aria-disabled])') || menu.querySelector('[role=menuitem]');
      if (eerste) eerste.focus();
    }
    function sluit(focusTerug) {
      if (menu.hidden) return;
      menu.hidden = true;
      document.querySelectorAll('tr.menu-open').forEach(function (r) { r.classList.remove('menu-open'); });
      if (focusTerug && terugFocus) terugFocus.focus();
    }

    paneel.addEventListener('contextmenu', function (e) {
      if (e.target.closest('input, textarea, select')) return;
      var rij = e.target.closest('tr[data-id]');
      e.preventDefault();
      terugFocus = rij ? (rij.querySelector('a.naam') || rij) : document.activeElement;
      var x = e.clientX, y = e.clientY;
      if (!x && !y) { var r = (rij || e.target).getBoundingClientRect(); x = r.left + 24; y = r.bottom; } // menutoets
      open(rij, x, y);
    });
    paneel.querySelectorAll('[data-menu]').forEach(function (k) {
      k.addEventListener('click', function (e) {
        e.stopPropagation();
        var r = k.getBoundingClientRect();
        terugFocus = k;
        open(k.closest('tr[data-id]'), r.left, r.bottom + 4);
      });
    });
    document.addEventListener('click', function (e) { if (!menu.contains(e.target)) sluit(false); });
    window.addEventListener('scroll', function () { sluit(false); }, true);
    window.addEventListener('resize', function () { sluit(false); });
    menu.addEventListener('keydown', function (e) {
      var items = Array.prototype.slice.call(menu.querySelectorAll('[role=menuitem]'));
      var i = items.indexOf(document.activeElement);
      if (e.key === 'ArrowDown') { e.preventDefault(); items[(i + 1) % items.length].focus(); }
      else if (e.key === 'ArrowUp') { e.preventDefault(); items[(i - 1 + items.length) % items.length].focus(); }
      else if (e.key === 'Home') { e.preventDefault(); items[0].focus(); }
      else if (e.key === 'End') { e.preventDefault(); items[items.length - 1].focus(); }
      else if (e.key === 'Escape' || e.key === 'Tab') { e.preventDefault(); sluit(true); }
      else if (e.key.length === 1) { // eerste letter
        var l = e.key.toLowerCase();
        for (var j = 1; j <= items.length; j++) { var it = items[(i + j) % items.length]; if (it.textContent.toLowerCase().charAt(0) === l) { it.focus(); break; } }
      }
    });
  }
})();
