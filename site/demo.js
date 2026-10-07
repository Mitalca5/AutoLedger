(function () {
  var root = document.getElementById('demo');
  if (!root) return;

  var CATS = ['energy', 'tolls', 'tires', 'maintenance', 'insurance', 'other'];
  var COLORS = { energy: '#38bdf8', tolls: '#f59e0b', tires: '#10b981', maintenance: '#ec4899', insurance: '#a855f7', other: '#64748b' };
  var FIXED = { insurance: 1, other: 1 };

  var STR = {
    en: {
      title: 'TCO dashboard', sub: 'Actual cost of ownership and cost per km for {v}', vehicle: 'Family wagon',
      hint: 'Sample data. Change the range or click a bar to open the monthly breakdown.',
      total: 'Total cost', perKm: 'Full cost per km', energy: 'Fuel', tolls: 'Tolls and parking',
      tires: 'Tires', maintenance: 'Maintenance', insurance: 'Insurance', other: 'Subscriptions, taxes and other',
      trend: 'Monthly expense trend (€)', breakdown: 'Full cost breakdown',
      r_all: 'All', r_12: '1 year', r_6: '6 months', r_3: '3 months', range: 'Time range',
      over: 'Over {d}', bar: '{m}: {a}. Open the breakdown.',
      mTitle: 'Cost breakdown', mSub: 'Every expense category and the cost per km',
      distance: 'Total distance', cost: 'Cost', fixedVar: 'Fixed vs variable costs', fixed: 'Fixed', variable: 'Variable',
      fixedHint: 'Insurance, taxes and subscriptions', varHint: 'Fuel, tolls, tires and maintenance',
      monthTotal: 'Month total', prev: 'Previous month', next: 'Next month', close: 'Close',
      chart: 'Monthly costs by category'
    },
    fr: {
      title: 'Tableau de bord TCO', sub: 'Coût réel de possession et coût au km pour {v}', vehicle: 'Break familial',
      hint: 'Données d’exemple. Changez la période ou cliquez sur une barre pour ouvrir le détail du mois.',
      total: 'Coût total', perKm: 'Coût complet au km', energy: 'Carburant', tolls: 'Péages et parkings',
      tires: 'Pneus', maintenance: 'Entretien', insurance: 'Assurance', other: 'Abonnements, taxes et autres',
      trend: 'Évolution mensuelle des dépenses (€)', breakdown: 'Répartition du coût complet',
      r_all: 'Tout', r_12: '1 an', r_6: '6 mois', r_3: '3 mois', range: 'Période',
      over: 'Sur {d}', bar: '{m} : {a}. Ouvrir le détail.',
      mTitle: 'Détail des coûts', mSub: 'Chaque poste de dépense et le coût au km',
      distance: 'Distance totale', cost: 'Coût', fixedVar: 'Coûts fixes et variables', fixed: 'Fixes', variable: 'Variables',
      fixedHint: 'Assurance, taxes et abonnements', varHint: 'Carburant, péages, pneus et entretien',
      monthTotal: 'Total du mois', prev: 'Mois précédent', next: 'Mois suivant', close: 'Fermer',
      chart: 'Coûts mensuels par poste'
    }
  };

  var START = new Date(2025, 4, 1);
  var KM = [1010, 1180, 1650, 1320, 980, 1105, 1240, 1420, 1190, 1060, 980, 1370, 1228, 1090, 1310, 1480, 1150, 1300];
  var ENERGY = [96, 108, 152, 121, 90, 102, 115, 131, 110, 98, 91, 128, 113, 100, 121, 138, 106, 119];
  var TOLLS = [8, 14, 52, 22, 0, 6, 18, 24, 12, 0, 0, 10, 0, 9, 16, 31, 7, 14];
  var MAINT = [0, 0, 0, 185, 0, 0, 0, 0, 64, 0, 0, 0, 0, 0, 310, 0, 0, 42];
  var DATA = KM.map(function (km, i) {
    return {
      date: new Date(START.getFullYear(), START.getMonth() + i, 1), km: km,
      energy: ENERGY[i], tolls: TOLLS[i], tires: Math.round(km * 0.0131 * 100) / 100,
      maintenance: MAINT[i], insurance: 62.5, other: 9
    };
  });
  DATA.forEach(function (m) { m.total = CATS.reduce(function (s, c) { return s + m[c]; }, 0); });

  var lang = 'en', range = 18, selected = -1, lastFocus = null;
  var NS = 'http://www.w3.org/2000/svg';

  function s(k, p) {
    var v = STR[lang][k];
    if (p) Object.keys(p).forEach(function (n) { v = v.replace('{' + n + '}', p[n]); });
    return v;
  }
  function loc() { return lang === 'fr' ? 'fr-FR' : 'en-US'; }
  function money(v, d) { return new Intl.NumberFormat(loc(), { style: 'currency', currency: 'EUR', minimumFractionDigits: d == null ? 2 : d, maximumFractionDigits: d == null ? 2 : d }).format(v); }
  function num(v) { return new Intl.NumberFormat(loc(), { maximumFractionDigits: 0 }).format(v); }
  function pct(v) { return new Intl.NumberFormat(loc(), { style: 'percent', maximumFractionDigits: 1 }).format(v); }
  function month(d, long) { return new Intl.DateTimeFormat(loc(), { month: long ? 'long' : 'short', year: long ? 'numeric' : '2-digit' }).format(d); }
  function cap(v) { return v.charAt(0).toUpperCase() + v.slice(1); }
  function esc(v) { return String(v).replace(/[&<>"]/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]; }); }

  function slice() { return DATA.slice(DATA.length - range); }
  function sum(rows, key) { return rows.reduce(function (a, r) { return a + r[key]; }, 0); }

  function donut(values, size, stroke) {
    var total = values.reduce(function (a, v) { return a + v.value; }, 0) || 1;
    var r = (size - stroke) / 2, c = 2 * Math.PI * r, off = 0, out = '';
    values.forEach(function (v) {
      var len = v.value / total * c;
      if (len > 0) out += '<circle cx="' + size / 2 + '" cy="' + size / 2 + '" r="' + r + '" fill="none" stroke="' + v.color + '" stroke-width="' + stroke + '" stroke-dasharray="' + len + ' ' + (c - len) + '" stroke-dashoffset="' + (-off) + '" transform="rotate(-90 ' + size / 2 + ' ' + size / 2 + ')"/>';
      off += len;
    });
    return '<svg viewBox="0 0 ' + size + ' ' + size + '" aria-hidden="true">' + out + '</svg>';
  }

  function chart(rows) {
    var W = 640, H = 230, L = 46, B = 28, T = 8, R = 6;
    var max = Math.max.apply(null, rows.map(function (r) { return r.total; }));
    var step = max > 300 ? 100 : 50, top = Math.ceil(max / step) * step;
    var pw = W - L - R, ph = H - T - B, bw = pw / rows.length, w = Math.min(bw * 0.62, 34);
    var g = '';
    for (var t = 0; t <= top; t += step) {
      var y = T + ph - t / top * ph;
      g += '<line x1="' + L + '" x2="' + (W - R) + '" y1="' + y + '" y2="' + y + '" class="gl"/><text x="' + (L - 8) + '" y="' + (y + 4) + '" text-anchor="end" class="ax">' + money(t, 0) + '</text>';
    }
    rows.forEach(function (r, i) {
      var idx = DATA.indexOf(r), x = L + i * bw + (bw - w) / 2, acc = 0, rects = '';
      CATS.forEach(function (c) {
        var h = r[c] / top * ph;
        if (h > 0) rects += '<rect x="' + x + '" y="' + (T + ph - (acc + r[c]) / top * ph) + '" width="' + w + '" height="' + h + '" fill="' + COLORS[c] + '"/>';
        acc += r[c];
      });
      var label = (rows.length <= 12 || i % 2 === 0) ? '<text x="' + (x + w / 2) + '" y="' + (H - 9) + '" text-anchor="middle" class="ax">' + esc(month(r.date)) + '</text>' : '';
      g += '<g class="bx' + (idx === selected ? ' on' : '') + '" tabindex="0" role="button" data-i="' + idx + '" aria-label="' + esc(s('bar', { m: cap(month(r.date, true)), a: money(r.total) })) + '">' +
        '<rect x="' + (L + i * bw) + '" y="' + T + '" width="' + bw + '" height="' + ph + '" class="hit"/>' + rects + label + '</g>';
    });
    return '<svg viewBox="0 0 ' + W + ' ' + H + '" class="trend" role="group" aria-label="' + esc(s('chart')) + '">' + g + '</svg>';
  }

  function legend() {
    return CATS.map(function (c) { return '<li><i style="background:' + COLORS[c] + '"></i>' + esc(s(c)) + '</li>'; }).join('');
  }

  function render() {
    var rows = slice(), km = sum(rows, 'km'), total = sum(rows, 'total');
    var ranges = [[18, 'r_all'], [12, 'r_12'], [6, 'r_6'], [3, 'r_3']];
    var pills = ranges.map(function (p) {
      return '<button type="button" data-r="' + p[0] + '" aria-pressed="' + (p[0] === range) + '">' + esc(s(p[1])) + '</button>';
    }).join('');
    var kpi = function (cls, label, value, note) {
      return '<div class="kpi ' + cls + '"><span>' + esc(label) + '</span><b>' + value + '</b><small>' + note + '</small></div>';
    };
    var parts = CATS.map(function (c) { return { value: sum(rows, c), color: COLORS[c] }; });
    var energyCost = sum(rows, 'energy');
    root.innerHTML =
      '<div class="d-head"><div><h3>' + esc(s('title')) + '</h3><p>' + esc(s('sub', { v: s('vehicle') })) + '</p></div><span class="d-tag">' + esc(s('vehicle')) + '</span></div>' +
      '<div class="d-kpis">' +
      kpi('', s('total'), money(total), esc(s('over', { d: num(km) + ' km' }))) +
      kpi('good', s('perKm'), money(total / km, 3) + '<em>/km</em>', esc(s('over', { d: num(km) + ' km' }))) +
      kpi('blue', s('energy'), money(energyCost), money(energyCost / km, 3) + '/km') +
      kpi('amber', s('tolls'), money(sum(rows, 'tolls')), money(sum(rows, 'tolls') / km, 3) + '/km') +
      '</div>' +
      '<div class="d-grid"><section class="card"><div class="card-h"><h4>' + esc(s('trend')) + '</h4><div class="pills" role="group" aria-label="' + esc(s('range')) + '">' + pills + '</div></div>' +
      chart(rows) + '<ul class="legend">' + legend() + '</ul></section>' +
      '<section class="card"><h4>' + esc(s('breakdown')) + '</h4><div class="don">' + donut(parts, 160, 26) + '</div>' +
      '<ul class="legend col">' + CATS.map(function (c, i) { return '<li><i style="background:' + COLORS[c] + '"></i>' + esc(s(c)) + '<b>' + pct(parts[i].value / total) + '</b></li>'; }).join('') + '</ul></section></div>' +
      '<p class="d-hint">' + esc(s('hint')) + '</p>' +
      '<div class="modal" hidden></div>';
  }

  function modal() {
    var box = root.querySelector('.modal');
    if (!box) return;
    if (selected < 0) { box.hidden = true; box.innerHTML = ''; return; }
    var m = DATA[selected], fixed = 0;
    CATS.forEach(function (c) { if (FIXED[c]) fixed += m[c]; });
    var variable = m.total - fixed, fp = fixed / m.total;
    var list = CATS.map(function (c) {
      var o = c === 'tires' ? '<span class="chip">' + (lang === 'fr' ? 'usure amortie' : 'amortised wear') + '</span>' : '';
      return '<li><i style="background:' + COLORS[c] + '"></i><div><strong>' + esc(s(c)) + o + '</strong></div><div class="r"><b>' + money(m[c]) + ' <small>(' + pct(m[c] / m.total) + ')</small></b><span>' + money(m[c] / m.km, 3) + '/km</span></div></li>';
    }).join('');
    box.hidden = false;
    box.innerHTML =
      '<div class="dlg" role="dialog" aria-modal="true" aria-labelledby="dlg-t">' +
      '<div class="dlg-h"><div><h3 id="dlg-t">' + esc(s('mTitle')) + ' — ' + esc(cap(month(m.date, true))) + '</h3><p>' + esc(s('mSub')) + '</p></div>' +
      '<div class="nav2"><button type="button" data-a="prev" aria-label="' + esc(s('prev')) + '"' + (selected === 0 ? ' disabled' : '') + '>‹</button>' +
      '<button type="button" data-a="next" aria-label="' + esc(s('next')) + '"' + (selected === DATA.length - 1 ? ' disabled' : '') + '>›</button>' +
      '<button type="button" data-a="close" aria-label="' + esc(s('close')) + '">✕</button></div></div>' +
      '<div class="d-kpis three"><div class="kpi"><span>' + esc(s('distance')) + '</span><b>' + num(m.km) + ' <em>km</em></b></div>' +
      '<div class="kpi good"><span>' + esc(s('perKm')) + '</span><b>' + money(m.total / m.km, 3) + '<em>/km</em></b></div>' +
      '<div class="kpi"><span>' + esc(s('cost')) + '</span><b>' + money(m.total) + '</b></div></div>' +
      '<div class="fv"><div class="fv-h"><span>' + esc(s('fixedVar')) + '</span><span><i class="fx"></i>' + esc(s('fixed')) + ' ' + pct(fp) + ' <i class="vr"></i>' + esc(s('variable')) + ' ' + pct(1 - fp) + '</span></div>' +
      '<div class="fv-b"><span style="width:' + (fp * 100) + '%"></span></div>' +
      '<div class="fv-f"><span>' + esc(s('fixedHint')) + '</span><span>' + esc(s('varHint')) + '</span></div></div>' +
      '<div class="dlg-body"><div class="don sm">' + donut(CATS.map(function (c) { return { value: m[c], color: COLORS[c] }; }), 150, 24) + '</div>' +
      '<ul class="rows">' + list + '<li class="tot"><div><strong>' + esc(s('monthTotal')) + '</strong> <small>(' + num(m.km) + ' km)</small></div><div class="r"><b>' + money(m.total) + '</b><span>' + money(m.total / m.km, 3) + '/km</span></div></li></ul></div></div>';
  }

  function open(i, from) {
    if (selected < 0) lastFocus = from || document.activeElement;
    selected = i;
    render(); modal();
    var f = root.querySelector('.dlg [data-a=close]');
    if (f) f.focus();
  }
  function close() {
    selected = -1;
    render();
    var back = lastFocus && lastFocus.getAttribute && lastFocus.getAttribute('data-i');
    var el = back != null ? root.querySelector('.bx[data-i="' + back + '"]') : null;
    if (el) el.focus();
  }

  root.addEventListener('click', function (e) {
    var t = e.target;
    var b = t.closest && t.closest('.bx');
    if (b) { open(+b.getAttribute('data-i'), b); return; }
    var r = t.closest && t.closest('[data-r]');
    if (r) { range = +r.getAttribute('data-r'); render(); var p = root.querySelector('[data-r="' + range + '"]'); if (p) p.focus(); return; }
    var a = t.closest && t.closest('[data-a]');
    if (a && !a.disabled) {
      var k = a.getAttribute('data-a');
      if (k === 'close') close();
      else open(selected + (k === 'next' ? 1 : -1));
      return;
    }
    if (t.classList && t.classList.contains('modal')) close();
  });
  root.addEventListener('keydown', function (e) {
    var t = e.target;
    if (e.key === 'Escape' && selected >= 0) { e.preventDefault(); close(); return; }
    if ((e.key === 'Enter' || e.key === ' ') && t.classList && t.classList.contains('bx')) { e.preventDefault(); open(+t.getAttribute('data-i'), t); return; }
    if (e.key === 'Tab' && selected >= 0) {
      var f = Array.prototype.filter.call(root.querySelectorAll('.dlg button'), function (x) { return !x.disabled; });
      if (!f.length) return;
      var first = f[0], last = f[f.length - 1];
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
      else if (!root.querySelector('.dlg').contains(document.activeElement)) { e.preventDefault(); first.focus(); }
    }
  });

  function setLang() {
    var l = (document.documentElement.lang || 'en').toLowerCase().indexOf('fr') === 0 ? 'fr' : 'en';
    if (l === lang && root.dataset.ready) return;
    lang = l; root.dataset.ready = '1';
    var keep = selected;
    render(); if (keep >= 0) modal();
  }
  new MutationObserver(setLang).observe(document.documentElement, { attributes: true, attributeFilter: ['lang'] });
  root.classList.add('live');
  setLang();
})();
