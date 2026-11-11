// Documenten: slepen, voortgang bij uploaden, bevestigen bij verwijderen.
// Zonder JavaScript werkt alles ook, alleen eenvoudiger.
document.querySelectorAll('form.upload').forEach(function (form) {
  var vak = form.querySelector('.vak');
  var invoer = form.querySelector('input[type=file]');
  var tekst = vak && vak.querySelector('span');
  var voortgang = form.querySelector('progress');
  var max = parseInt(form.dataset.max || '0', 10) * 1048576;
  if (!vak || !invoer) return;
  ['dragenter', 'dragover'].forEach(function (e) { vak.addEventListener(e, function (ev) { ev.preventDefault(); vak.classList.add('erboven'); }); });
  ['dragleave', 'drop'].forEach(function (e) { vak.addEventListener(e, function () { vak.classList.remove('erboven'); }); });
  vak.addEventListener('drop', function (ev) {
    ev.preventDefault();
    if (ev.dataTransfer && ev.dataTransfer.files.length) { invoer.files = ev.dataTransfer.files; invoer.dispatchEvent(new Event('change')); }
  });
  invoer.addEventListener('change', function () {
    var n = invoer.files.length;
    if (tekst && n) tekst.textContent = n === 1 ? invoer.files[0].name : n + ' bestanden gekozen';
  });
  form.addEventListener('submit', function (ev) {
    if (!window.XMLHttpRequest || !voortgang) return;
    for (var i = 0; i < invoer.files.length; i++) {
      if (max && invoer.files[i].size > max) { ev.preventDefault(); alert(invoer.files[i].name + ' is groter dan ' + form.dataset.max + ' MB.'); return; }
    }
    ev.preventDefault();
    var xhr = new XMLHttpRequest();
    xhr.open('POST', form.action);
    xhr.upload.onprogress = function (e) { if (e.lengthComputable) voortgang.value = Math.round(e.loaded / e.total * 100); };
    xhr.onload = function () { window.location.href = xhr.responseURL || window.location.href; };
    xhr.onerror = function () { voortgang.hidden = true; alert('De upload is niet gelukt. Probeer het opnieuw.'); };
    voortgang.hidden = false;
    form.querySelector('button').disabled = true;
    xhr.send(new FormData(form));
  });
});
document.querySelectorAll('form[data-bevestig]').forEach(function (form) {
  form.addEventListener('submit', function (ev) { if (!confirm(form.dataset.bevestig)) ev.preventDefault(); });
});
