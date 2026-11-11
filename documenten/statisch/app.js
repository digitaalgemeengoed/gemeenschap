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

  // Een recht direct opslaan bij het kiezen.
  document.querySelectorAll('select[data-direct]').forEach(function (s) {
    s.addEventListener('change', function () { s.form.submit(); });
  });
})();
