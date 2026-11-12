---
title: "Certificate Expiry Tracker"
description: "Track SSL certificate expiry dates in your browser. Paste PEM certs to monitor when they expire — stored in localStorage, nothing sent to any server."
status: "live"
icon: "📋"
tags: ["ssl", "tls", "certificates", "pki", "expiry", "monitoring", "x509"]
weight: 3
---

<div class="trk-app">

  <div class="page-hero">
    <h1>Certificate Expiry Tracker</h1>
    <p>Paste PEM certificates to monitor expiry dates. Everything is stored locally in your browser — nothing is sent to any server.</p>
  </div>

  <!-- Summary bar — hidden until at least one cert is tracked -->
  <div id="trk-summary" class="trk-summary" style="display:none">
    <span class="trk-sum-item trk-sum-expired"  id="trk-cnt-expired"></span>
    <span class="trk-sum-item trk-sum-critical" id="trk-cnt-critical"></span>
    <span class="trk-sum-item trk-sum-warning"  id="trk-cnt-warning"></span>
    <span class="trk-sum-item trk-sum-valid"    id="trk-cnt-valid"></span>
    <button id="trk-clear-all" class="trk-clear-all-btn">Clear all</button>
  </div>

  <!-- Tracker list -->
  <div id="trk-list-wrap" style="display:none">
    <div class="trk-card">
      <div class="trk-card-header">
        <h2>Tracked Certificates</h2>
        <div class="trk-sort-row">
          <span class="trk-sort-label">Sort by</span>
          <button class="trk-sort-btn trk-sort-active" data-sort="expiry">Expiry</button>
          <button class="trk-sort-btn" data-sort="label">Label</button>
          <button class="trk-sort-btn" data-sort="added">Added</button>
        </div>
      </div>
      <div id="trk-list" class="trk-list"></div>
    </div>
  </div>

  <!-- Empty state -->
  <div id="trk-empty" class="trk-empty">
    <div class="trk-empty-icon">📋</div>
    <p class="trk-empty-title">No certificates tracked yet</p>
    <p class="trk-empty-hint">Paste a PEM certificate below to start tracking its expiry.</p>
  </div>

  <!-- Add certificate -->
  <div class="trk-card">
    <div class="trk-card-header"><h2>Add Certificate</h2></div>
    <div class="trk-add-body">
      <div class="trk-add-label-row">
        <label class="trk-label" for="trk-label-in">Label (optional)</label>
        <input id="trk-label-in" class="trk-input" type="text" placeholder="e.g. api.example.com prod" autocomplete="off" spellcheck="false" />
      </div>
      <label class="trk-label">PEM Certificate</label>
      <textarea id="trk-pem-in" class="trk-textarea" spellcheck="false" rows="7" placeholder="-----BEGIN CERTIFICATE-----&#10;MIIFazCCA1Og...&#10;-----END CERTIFICATE-----&#10;&#10;Paste one or more PEM certificates. Multiple certs are added individually."></textarea>
      <div class="trk-add-footer">
        <div class="trk-get-cert-hint">
          <span class="trk-hint-label">Get cert from a live server:</span>
          <code class="trk-hint-code">echo | openssl s_client -connect example.com:443 2&gt;/dev/null | openssl x509</code>
        </div>
        <button id="trk-add-btn" class="trk-add-btn">Add →</button>
      </div>
    </div>
  </div>

  <div id="trk-add-error" class="trk-error" style="display:none"></div>

</div>

<style>
.tool-page-container { max-width: 960px; }
.page-hero { margin-bottom: 0.25rem; }
.page-hero h1 { color: var(--fg, #e2e8f0); font-size: 2rem; font-weight: 800; letter-spacing: -0.02em; margin-bottom: 0.35rem; }
.page-hero p  { color: var(--muted, #94a3b8); font-size: 0.95rem; }

.trk-app { display: flex; flex-direction: column; gap: 1.5rem; }

/* Summary bar */
.trk-summary { align-items: center; background: var(--card, #1a2233); border: 1px solid var(--border, #2d3a4f); border-radius: 10px; display: flex; flex-wrap: wrap; gap: 0.75rem; padding: 0.85rem 1.5rem; }
.trk-sum-item { border-radius: 5px; font-size: 0.78rem; font-weight: 700; padding: 0.25rem 0.65rem; display: none; }
.trk-sum-item.visible { display: inline-block; }
.trk-sum-expired  { background: rgba(239,68,68,0.12);  border: 1px solid rgba(239,68,68,0.3);  color: #f87171; }
.trk-sum-critical { background: rgba(239,68,68,0.08);  border: 1px solid rgba(239,68,68,0.2);  color: #fca5a5; }
.trk-sum-warning  { background: rgba(245,158,11,0.1);  border: 1px solid rgba(245,158,11,0.3); color: #fbbf24; }
.trk-sum-valid    { background: rgba(16,185,129,0.1);  border: 1px solid rgba(16,185,129,0.3); color: #34d399; }
.trk-clear-all-btn { background: transparent; border: 1px solid var(--border, #2d3a4f); border-radius: 6px; color: var(--muted, #94a3b8); cursor: pointer; font-size: 0.75rem; margin-left: auto; padding: 0.25rem 0.75rem; transition: color 0.15s, border-color 0.15s; }
.trk-clear-all-btn:hover { border-color: rgba(239,68,68,0.5); color: #f87171; }

/* Card */
.trk-card { background: var(--card, #1a2233); border: 1px solid var(--border, #2d3a4f); border-radius: 10px; overflow: hidden; }
.trk-card-header { align-items: center; border-bottom: 1px solid var(--border, #2d3a4f); display: flex; gap: 1rem; justify-content: space-between; padding: 1rem 1.5rem; flex-wrap: wrap; }
.trk-card-header h2 { color: var(--fg, #e2e8f0); font-size: 1rem; font-weight: 700; margin: 0; }

/* Sort buttons */
.trk-sort-row { align-items: center; display: flex; gap: 0.4rem; }
.trk-sort-label { color: var(--muted, #94a3b8); font-size: 0.72rem; font-weight: 700; letter-spacing: 0.05em; }
.trk-sort-btn { background: transparent; border: 1px solid var(--border, #2d3a4f); border-radius: 6px; color: var(--muted, #94a3b8); cursor: pointer; font-size: 0.75rem; padding: 0.2rem 0.6rem; transition: color 0.15s, border-color 0.15s, background 0.15s; }
.trk-sort-btn:hover { color: var(--fg, #e2e8f0); }
.trk-sort-active { background: rgba(59,130,246,0.1); border-color: rgba(59,130,246,0.4); color: var(--fg, #e2e8f0); }

/* Tracker rows */
.trk-list { display: flex; flex-direction: column; }
.trk-row { align-items: center; border-bottom: 1px solid var(--border, #2d3a4f); display: grid; gap: 0; grid-template-columns: 100px 1fr auto auto 110px 32px; padding: 0.75rem 1.5rem; transition: background 0.1s; }
.trk-row:last-child { border-bottom: none; }
.trk-row:hover { background: rgba(255,255,255,0.02); }

.trk-status-badge { border-radius: 5px; font-size: 0.68rem; font-weight: 700; padding: 0.2rem 0.5rem; text-align: center; white-space: nowrap; }
.trk-st-expired  { background: rgba(239,68,68,0.15);  border: 1px solid rgba(239,68,68,0.35);  color: #f87171; }
.trk-st-critical { background: rgba(239,68,68,0.1);   border: 1px solid rgba(239,68,68,0.25);  color: #fca5a5; }
.trk-st-warning  { background: rgba(245,158,11,0.1);  border: 1px solid rgba(245,158,11,0.3);  color: #fbbf24; }
.trk-st-soon     { background: rgba(251,191,36,0.08); border: 1px solid rgba(251,191,36,0.2);  color: #fcd34d; }
.trk-st-valid    { background: rgba(16,185,129,0.1);  border: 1px solid rgba(16,185,129,0.25); color: #34d399; }

.trk-info-col { min-width: 0; padding: 0 1rem; }
.trk-label-text { color: var(--fg, #e2e8f0); font-size: 0.88rem; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.trk-cn-text { color: var(--muted, #94a3b8); font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.75rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.trk-sans-text { color: var(--muted, #94a3b8); font-size: 0.72rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin-top: 0.1rem; }

.trk-issuer-col { color: var(--muted, #94a3b8); font-size: 0.75rem; overflow: hidden; padding: 0 1rem; text-overflow: ellipsis; white-space: nowrap; max-width: 180px; }

.trk-expiry-col { font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.78rem; padding: 0 0 0 0.5rem; text-align: right; white-space: nowrap; }
.trk-expiry-date { color: var(--fg, #e2e8f0); display: block; }
.trk-expiry-days { color: var(--muted, #94a3b8); display: block; font-size: 0.7rem; margin-top: 0.1rem; }
.trk-expiry-days.trk-days-expired  { color: #f87171; }
.trk-expiry-days.trk-days-critical { color: #fca5a5; }
.trk-expiry-days.trk-days-warning  { color: #fbbf24; }
.trk-expiry-days.trk-days-soon     { color: #fcd34d; }
.trk-expiry-days.trk-days-valid    { color: #34d399; }

.trk-del-btn { background: transparent; border: none; border-radius: 5px; color: var(--muted, #94a3b8); cursor: pointer; font-size: 0.95rem; height: 28px; line-height: 1; padding: 0; transition: color 0.15s; width: 28px; }
.trk-del-btn:hover { color: #f87171; }

/* Empty state */
.trk-empty { align-items: center; display: flex; flex-direction: column; gap: 0.5rem; padding: 2.5rem 1.5rem; text-align: center; }
.trk-empty-icon { font-size: 2.5rem; line-height: 1; opacity: 0.3; }
.trk-empty-title { color: var(--fg, #e2e8f0); font-size: 1rem; font-weight: 600; margin: 0; }
.trk-empty-hint  { color: var(--muted, #94a3b8); font-size: 0.875rem; margin: 0; }

/* Add form */
.trk-add-body { display: flex; flex-direction: column; gap: 0.75rem; padding: 1.25rem 1.5rem; }
.trk-add-label-row { display: flex; flex-direction: column; gap: 0.35rem; }
.trk-label { color: var(--muted, #94a3b8); font-size: 0.72rem; font-weight: 700; letter-spacing: 0.04em; }
.trk-input { background: #0d1420; border: 1px solid var(--border, #2d3a4f); border-radius: 7px; color: var(--fg, #e2e8f0); font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.88rem; outline: none; padding: 0.5rem 0.9rem; transition: border-color 0.15s; }
.trk-input:focus { border-color: var(--accent, #3b82f6); }
.trk-textarea { background: #0d1420; border: 1px solid var(--border, #2d3a4f); border-radius: 7px; box-sizing: border-box; color: var(--fg, #e2e8f0); display: block; font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.8rem; line-height: 1.6; outline: none; padding: 0.65rem 0.9rem; resize: vertical; transition: border-color 0.15s; width: 100%; }
.trk-textarea:focus { border-color: var(--accent, #3b82f6); }

.trk-add-footer { align-items: flex-start; display: flex; gap: 1rem; justify-content: space-between; flex-wrap: wrap; }
.trk-get-cert-hint { display: flex; flex-direction: column; gap: 0.3rem; flex: 1; min-width: 0; }
.trk-hint-label { color: var(--muted, #94a3b8); font-size: 0.72rem; font-weight: 700; letter-spacing: 0.04em; }
.trk-hint-code { background: rgba(0,0,0,0.3); border: 1px solid var(--border, #2d3a4f); border-radius: 5px; color: #93c5fd; font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.75rem; overflow-x: auto; padding: 0.3rem 0.65rem; white-space: nowrap; }

.trk-add-btn { background: var(--accent, #3b82f6); border: none; border-radius: 7px; color: #fff; cursor: pointer; flex-shrink: 0; font-size: 0.9rem; font-weight: 700; padding: 0.6rem 1.5rem; transition: background 0.15s; white-space: nowrap; align-self: flex-end; }
.trk-add-btn:hover { background: #2563eb; }

.trk-error { background: rgba(239,68,68,0.08); border: 1px solid rgba(239,68,68,0.25); border-radius: 10px; color: #fca5a5; font-size: 0.875rem; padding: 0.9rem 1.25rem; }

/* Mobile */
@media (max-width: 700px) {
  .page-hero h1 { font-size: 1.6rem; }
  .trk-row { grid-template-columns: 80px 1fr 90px 28px; }
  .trk-issuer-col { display: none; }
  .trk-expiry-col { padding-left: 0.25rem; }
}
@media (max-width: 480px) {
  .trk-row { grid-template-columns: 1fr 90px 28px; }
  .trk-status-badge { display: none; }
}
</style>

<script>
(function () {
  'use strict';

  var STORE_KEY = 'deploycraft_cert_tracker_v1';

  // ── Minimal ASN.1/DER decoder (subject, issuer, SANs, validity) ──────────────

  function parseDER(buf, off) {
    var v = new Uint8Array(buf), tb = v[off], pos = off + 1;
    var tagClass = (tb >> 6) & 3, constructed = !!(tb & 0x20), tag = tb & 0x1f;
    if (tag === 0x1f) { tag = 0; while (pos < v.length) { var b = v[pos++]; tag = (tag << 7)|(b&0x7f); if (!(b&0x80)) break; } }
    var len = 0, lb = v[pos++];
    if (lb < 0x80) { len = lb; } else { var n = lb & 0x7f; for (var i = 0; i < n; i++) len = (len * 256) + v[pos++]; }
    return { tag: tag, tagClass: tagClass, constructed: constructed, length: len, vs: pos, ve: pos+len, end: pos+len };
  }

  function readChildren(buf, start, end) {
    var c = [], pos = start;
    while (pos < end) { var n = parseDER(buf, pos); c.push(n); pos = n.end; }
    return c;
  }

  function decodeOID(buf, n) {
    var bytes = new Uint8Array(buf, n.vs, n.length), parts = [], v = 0, first = true;
    for (var i = 0; i < bytes.length; i++) {
      var b = bytes[i]; v = v * 128 + (b & 0x7f);
      if (!(b & 0x80)) { if (first) { parts.push(Math.floor(v/40), v%40); first = false; } else parts.push(v); v = 0; }
    }
    return parts.join('.');
  }

  function decodeStr(buf, n) {
    var bytes = new Uint8Array(buf, n.vs, n.length);
    if (n.tag === 0x1e) { var s = ''; for (var i = 0; i < bytes.length; i+=2) s += String.fromCharCode((bytes[i]<<8)|bytes[i+1]); return s; }
    return new TextDecoder('utf-8', { fatal: false }).decode(bytes);
  }

  function decodeTime(buf, n) {
    var str = new TextDecoder().decode(new Uint8Array(buf, n.vs, n.length));
    var yr, mo, dy, hh, mm, ss;
    if (n.tag === 23) {
      yr = parseInt(str.slice(0,2),10); yr += yr < 50 ? 2000 : 1900;
      mo = parseInt(str.slice(2,4),10); dy = parseInt(str.slice(4,6),10);
      hh = parseInt(str.slice(6,8),10); mm = parseInt(str.slice(8,10),10); ss = parseInt(str.slice(10,12),10);
    } else {
      yr = parseInt(str.slice(0,4),10); mo = parseInt(str.slice(4,6),10); dy = parseInt(str.slice(6,8),10);
      hh = parseInt(str.slice(8,10),10); mm = parseInt(str.slice(10,12),10); ss = parseInt(str.slice(12,14),10);
    }
    return new Date(Date.UTC(yr, mo-1, dy, hh, mm, ss));
  }

  var ATTR_OIDS = { '2.5.4.3':'CN','2.5.4.6':'C','2.5.4.7':'L','2.5.4.8':'ST','2.5.4.10':'O','2.5.4.11':'OU' };

  function decodeRDN(buf, node) {
    var rdns = {}, sets = readChildren(buf, node.vs, node.ve);
    for (var i = 0; i < sets.length; i++) {
      var attrs = readChildren(buf, sets[i].vs, sets[i].ve);
      for (var j = 0; j < attrs.length; j++) {
        var parts = readChildren(buf, attrs[j].vs, attrs[j].ve);
        if (parts.length < 2) continue;
        var name = ATTR_OIDS[decodeOID(buf, parts[0])] || decodeOID(buf, parts[0]);
        rdns[name] = decodeStr(buf, parts[1]);
      }
    }
    return rdns;
  }

  function decodeSAN(buf, octetNode) {
    try {
      var seq = parseDER(buf, octetNode.vs);
      if (seq.tag !== 0x10) return [];
      var gns = readChildren(buf, seq.vs, seq.ve), names = [];
      for (var i = 0; i < gns.length; i++) {
        var gn = gns[i], bytes = new Uint8Array(buf, gn.vs, gn.length);
        if (gn.tag === 2) names.push(new TextDecoder().decode(bytes));
        else if (gn.tag === 7 && bytes.length === 4) names.push(Array.from(bytes).join('.'));
      }
      return names;
    } catch (e) { return []; }
  }

  function pemToDER(pem) {
    var b64 = pem.replace(/-----[^-]+-----/g,'').replace(/\s+/g,'');
    var bin = atob(b64), buf = new ArrayBuffer(bin.length), v = new Uint8Array(buf);
    for (var i = 0; i < bin.length; i++) v[i] = bin.charCodeAt(i);
    return buf;
  }

  function extractCertInfo(pem) {
    var buf = pemToDER(pem);
    var certSeq = parseDER(buf, 0);
    var cc = readChildren(buf, certSeq.vs, certSeq.ve);
    var tbs = cc[0];
    var tc = readChildren(buf, tbs.vs, tbs.ve);

    var idx = 0;
    if (tc[idx].tagClass === 2 && tc[idx].tag === 0) idx++;
    idx++; // serial
    idx++; // sigAlg
    var issuerNode = tc[idx++];
    var validNode  = tc[idx++];
    var subjNode   = tc[idx++];
    idx++; // spki

    while (idx < tc.length && tc[idx].tagClass === 2 && tc[idx].tag < 3) idx++;

    var sans = [];
    if (idx < tc.length && tc[idx].tagClass === 2 && tc[idx].tag === 3) {
      var extSeq = parseDER(buf, tc[idx].vs);
      var exts = readChildren(buf, extSeq.vs, extSeq.ve);
      for (var xi = 0; xi < exts.length; xi++) {
        var ep = readChildren(buf, exts[xi].vs, exts[xi].ve);
        if (ep.length < 2) continue;
        if (decodeOID(buf, ep[0]) === '2.5.29.17') {
          sans = decodeSAN(buf, ep[ep.length - 1]);
        }
      }
    }

    var vc = readChildren(buf, validNode.vs, validNode.ve);
    var subj   = decodeRDN(buf, subjNode);
    var issuer = decodeRDN(buf, issuerNode);

    return {
      cn:        subj.CN || subj.O || '(no CN)',
      issuerCN:  issuer.CN || issuer.O || '—',
      sans:      sans,
      notBefore: decodeTime(buf, vc[0]),
      notAfter:  decodeTime(buf, vc[1]),
    };
  }

  function extractAllPEMs(text) {
    var re = /-----BEGIN CERTIFICATE-----[\s\S]+?-----END CERTIFICATE-----/g, m, pems = [];
    while ((m = re.exec(text)) !== null) pems.push(m[0]);
    return pems;
  }

  // ── Status helpers ────────────────────────────────────────────────────────────

  function statusFor(notAfter) {
    var days = Math.floor((notAfter.getTime() - Date.now()) / 86400000);
    if (days < 0)  return { key: 'expired',  label: 'EXPIRED',   days: days };
    if (days < 14) return { key: 'critical', label: '< 14 DAYS', days: days };
    if (days < 30) return { key: 'warning',  label: '< 30 DAYS', days: days };
    if (days < 90) return { key: 'soon',     label: '< 90 DAYS', days: days };
    return           { key: 'valid',    label: 'VALID',      days: days };
  }

  function daysLabel(st) {
    if (st.days < 0) return 'Expired ' + Math.abs(st.days) + 'd ago';
    if (st.days === 0) return 'Expires today';
    return st.days + 'd remaining';
  }

  function formatDate(d) {
    return d.toISOString().slice(0, 10);
  }

  // ── Storage ───────────────────────────────────────────────────────────────────

  function load() {
    try { return JSON.parse(localStorage.getItem(STORE_KEY) || '[]'); }
    catch (e) { return []; }
  }

  function save(list) {
    try { localStorage.setItem(STORE_KEY, JSON.stringify(list)); } catch (e) {}
  }

  function uid() {
    return (crypto.randomUUID ? crypto.randomUUID() : Date.now().toString(36) + Math.random().toString(36).slice(2));
  }

  // ── Sort ──────────────────────────────────────────────────────────────────────

  var currentSort = 'expiry';

  function sortList(list) {
    var copy = list.slice();
    if (currentSort === 'expiry') copy.sort(function(a,b) { return new Date(a.notAfter) - new Date(b.notAfter); });
    else if (currentSort === 'label') copy.sort(function(a,b) { return (a.label||a.cn).localeCompare(b.label||b.cn); });
    else copy.sort(function(a,b) { return new Date(b.addedAt) - new Date(a.addedAt); });
    return copy;
  }

  // ── Render ────────────────────────────────────────────────────────────────────

  function render() {
    var list = sortList(load());
    var listEl  = document.getElementById('trk-list');
    var wrapEl  = document.getElementById('trk-list-wrap');
    var emptyEl = document.getElementById('trk-empty');
    var sumEl   = document.getElementById('trk-summary');

    if (!list.length) {
      wrapEl.style.display = 'none';
      emptyEl.style.display = '';
      sumEl.style.display = 'none';
      return;
    }

    emptyEl.style.display = 'none';
    wrapEl.style.display = '';
    sumEl.style.display = '';

    // Summary counts
    var counts = { expired: 0, critical: 0, warning: 0, valid: 0 };
    list.forEach(function(c) {
      var st = statusFor(new Date(c.notAfter));
      if (st.key === 'expired')  counts.expired++;
      else if (st.key === 'critical') counts.critical++;
      else if (st.key === 'warning' || st.key === 'soon') counts.warning++;
      else counts.valid++;
    });

    function setSumItem(id, cls, n, label) {
      var el = document.getElementById(id);
      if (n > 0) { el.textContent = n + ' ' + label; el.classList.add('visible'); }
      else        { el.textContent = ''; el.classList.remove('visible'); }
    }
    setSumItem('trk-cnt-expired',  'trk-sum-expired',  counts.expired,  counts.expired  === 1 ? 'expired'    : 'expired');
    setSumItem('trk-cnt-critical', 'trk-sum-critical', counts.critical, counts.critical === 1 ? 'critical'   : 'critical');
    setSumItem('trk-cnt-warning',  'trk-sum-warning',  counts.warning,  counts.warning  === 1 ? 'expiring'   : 'expiring');
    setSumItem('trk-cnt-valid',    'trk-sum-valid',    counts.valid,    counts.valid    === 1 ? 'valid'       : 'valid');

    // Rows
    listEl.innerHTML = '';
    list.forEach(function(cert) {
      var na = new Date(cert.notAfter);
      var st = statusFor(na);
      var label = cert.label || cert.cn;
      var sansStr = cert.sans && cert.sans.length ? cert.sans.slice(0, 4).join(', ') + (cert.sans.length > 4 ? ' …' : '') : '';

      var row = document.createElement('div');
      row.className = 'trk-row';

      row.innerHTML =
        '<span class="trk-status-badge trk-st-' + st.key + '">' + st.label + '</span>' +
        '<div class="trk-info-col">' +
          '<div class="trk-label-text" title="' + esc(label) + '">' + esc(label) + '</div>' +
          (cert.label && cert.label !== cert.cn ? '<div class="trk-cn-text" title="' + esc(cert.cn) + '">' + esc(cert.cn) + '</div>' : '') +
          (sansStr ? '<div class="trk-sans-text" title="' + esc(cert.sans.join(', ')) + '">' + esc(sansStr) + '</div>' : '') +
        '</div>' +
        '<div class="trk-issuer-col" title="' + esc(cert.issuerCN) + '">' + esc(cert.issuerCN) + '</div>' +
        '<div class="trk-expiry-col">' +
          '<span class="trk-expiry-date">' + formatDate(na) + '</span>' +
          '<span class="trk-expiry-days trk-days-' + st.key + '">' + daysLabel(st) + '</span>' +
        '</div>' +
        '<button class="trk-del-btn" data-id="' + cert.id + '" title="Remove">✕</button>';

      listEl.appendChild(row);
    });
  }

  function esc(s) {
    return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
  }

  // ── Add ───────────────────────────────────────────────────────────────────────

  function addCerts() {
    var pemText  = document.getElementById('trk-pem-in').value.trim();
    var labelVal = document.getElementById('trk-label-in').value.trim();
    var errorEl  = document.getElementById('trk-add-error');
    errorEl.style.display = 'none';

    if (!pemText) return;

    var pems = extractAllPEMs(pemText);
    if (!pems.length) {
      errorEl.textContent = '⚠ No valid PEM certificate found. Make sure it starts with -----BEGIN CERTIFICATE-----.';
      errorEl.style.display = '';
      return;
    }

    var list = load();
    var added = 0, errors = [];

    pems.forEach(function(pem, i) {
      try {
        var info = extractCertInfo(pem);
        list.push({
          id:        uid(),
          label:     pems.length === 1 ? (labelVal || info.cn) : (labelVal ? labelVal + ' (' + (i+1) + ')' : info.cn),
          cn:        info.cn,
          issuerCN:  info.issuerCN,
          sans:      info.sans,
          notBefore: info.notBefore.toISOString(),
          notAfter:  info.notAfter.toISOString(),
          addedAt:   new Date().toISOString(),
        });
        added++;
      } catch (e) {
        errors.push('Certificate ' + (i+1) + ': ' + (e.message || 'parse error'));
      }
    });

    if (!added && errors.length) {
      errorEl.textContent = '⚠ ' + errors[0];
      errorEl.style.display = '';
      return;
    }

    save(list);
    document.getElementById('trk-pem-in').value   = '';
    document.getElementById('trk-label-in').value = '';

    if (errors.length) {
      errorEl.textContent = '⚠ Added ' + added + ' cert(s). ' + errors.length + ' failed: ' + errors[0];
      errorEl.style.display = '';
    }

    render();
    document.getElementById('trk-list-wrap').scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  }

  // ── Events ────────────────────────────────────────────────────────────────────

  document.getElementById('trk-add-btn').addEventListener('click', addCerts);

  document.getElementById('trk-pem-in').addEventListener('keydown', function(e) {
    if (e.key === 'Enter' && e.ctrlKey) addCerts();
  });

  document.getElementById('trk-list').addEventListener('click', function(e) {
    var btn = e.target.closest('.trk-del-btn');
    if (!btn) return;
    var list = load().filter(function(c) { return c.id !== btn.dataset.id; });
    save(list);
    render();
  });

  document.getElementById('trk-clear-all').addEventListener('click', function() {
    if (!confirm('Remove all tracked certificates?')) return;
    save([]);
    render();
  });

  document.getElementById('trk-sort-row') && document.querySelectorAll('.trk-sort-btn').forEach(function(btn) {
    btn.addEventListener('click', function() {
      document.querySelectorAll('.trk-sort-btn').forEach(function(b) { b.classList.remove('trk-sort-active'); });
      btn.classList.add('trk-sort-active');
      currentSort = btn.dataset.sort;
      render();
    });
  });

  // Wire sort buttons (the forEach above may miss trk-sort-row check)
  document.querySelectorAll('.trk-sort-btn').forEach(function(btn) {
    btn.addEventListener('click', function() {
      document.querySelectorAll('.trk-sort-btn').forEach(function(b) { b.classList.remove('trk-sort-active'); });
      btn.classList.add('trk-sort-active');
      currentSort = btn.dataset.sort;
      render();
    });
  });

  // Auto-add on paste into the textarea
  document.getElementById('trk-pem-in').addEventListener('paste', function() {
    setTimeout(function() {
      var text = document.getElementById('trk-pem-in').value;
      if (/-----BEGIN CERTIFICATE-----/.test(text)) addCerts();
    }, 80);
  });

  render();
})();
</script>
