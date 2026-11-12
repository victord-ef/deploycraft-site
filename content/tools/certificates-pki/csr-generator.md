---
title: "CSR Generator"
description: "Generate a Certificate Signing Request (CSR) and private key pair in your browser — RSA or ECDSA, with SANs. Nothing is sent to any server."
status: "live"
icon: "🔑"
tags: ["csr", "certificates", "pki", "rsa", "ecdsa", "tls", "ssl", "x509"]
weight: 2
---

<div class="csr-app">

  <div class="page-hero">
    <h1>CSR Generator</h1>
    <p>Generate a Certificate Signing Request and private key pair entirely in your browser — RSA 2048/4096 or ECDSA P-256/P-384. Nothing is sent to any server.</p>
  </div>

  <div class="csr-card">
    <div class="csr-card-header"><h2>Certificate Details</h2></div>
    <div class="csr-form-body">
      <div class="csr-fields-grid">
        <div class="csr-field csr-field-wide">
          <label class="csr-label" for="csr-cn">Common Name (CN) <span class="csr-req">*</span></label>
          <input id="csr-cn" class="csr-input" type="text" placeholder="example.com" autocomplete="off" spellcheck="false" />
          <span class="csr-field-hint">Domain name, service identifier, or wildcard (*.example.com)</span>
        </div>
        <div class="csr-field">
          <label class="csr-label" for="csr-o">Organization (O)</label>
          <input id="csr-o" class="csr-input" type="text" placeholder="My Company Ltd." autocomplete="off" spellcheck="false" />
        </div>
        <div class="csr-field">
          <label class="csr-label" for="csr-ou">Org. Unit (OU)</label>
          <input id="csr-ou" class="csr-input" type="text" placeholder="Engineering" autocomplete="off" spellcheck="false" />
        </div>
        <div class="csr-field">
          <label class="csr-label" for="csr-l">City / Locality (L)</label>
          <input id="csr-l" class="csr-input" type="text" placeholder="London" autocomplete="off" spellcheck="false" />
        </div>
        <div class="csr-field">
          <label class="csr-label" for="csr-st">State / Province (ST)</label>
          <input id="csr-st" class="csr-input" type="text" placeholder="England" autocomplete="off" spellcheck="false" />
        </div>
        <div class="csr-field">
          <label class="csr-label" for="csr-c">Country (C)</label>
          <input id="csr-c" class="csr-input" type="text" placeholder="GB" maxlength="2" style="text-transform:uppercase" autocomplete="off" spellcheck="false" />
          <span class="csr-field-hint">2-letter ISO code — US, GB, DE…</span>
        </div>
      </div>
    </div>
  </div>

  <div class="csr-card">
    <div class="csr-card-header">
      <h2>Subject Alternative Names</h2>
      <span class="csr-header-hint">One name per line</span>
    </div>
    <div class="csr-san-body">
      <textarea id="csr-sans" class="csr-textarea" spellcheck="false" rows="4" placeholder="example.com&#10;*.example.com&#10;www.example.com&#10;192.168.1.10"></textarea>
      <p class="csr-san-note">For TLS certificates, add the CN here too — modern browsers require the domain in the SANs, not just the CN.</p>
    </div>
  </div>

  <div class="csr-card">
    <div class="csr-card-header"><h2>Key Type</h2></div>
    <div class="csr-key-body">
      <div class="csr-key-opts" id="csr-key-opts">
        <button class="csr-key-opt csr-key-active" data-val="ec-p256">EC P-256 <span class="csr-rec-badge">recommended</span></button>
        <button class="csr-key-opt" data-val="ec-p384">EC P-384</button>
        <button class="csr-key-opt" data-val="rsa-2048">RSA 2048</button>
        <button class="csr-key-opt" data-val="rsa-4096">RSA 4096 <span class="csr-slow-badge">slow</span></button>
      </div>
      <p class="csr-key-note" id="csr-key-note">P-256: 128-bit security, fastest signing, smallest signatures. Supported by all modern TLS stacks.</p>
    </div>
  </div>

  <div id="csr-error" class="csr-error" style="display:none"></div>

  <button id="csr-gen-btn" class="csr-gen-btn">Generate CSR + Private Key</button>

  <div id="csr-loading" class="csr-loading" style="display:none">
    <span class="csr-spinner"></span>
    <span id="csr-loading-msg">Generating key pair…</span>
  </div>

  <div id="csr-output" style="display:none">

    <div class="csr-card">
      <div class="csr-card-header">
        <h2>Certificate Signing Request</h2>
        <div class="csr-out-actions">
          <button class="csr-copy-btn" data-target="csr-csr-pre">Copy</button>
          <button class="csr-dl-btn" data-target="csr-csr-pre" data-name="request.csr">Download .csr</button>
        </div>
      </div>
      <pre id="csr-csr-pre" class="csr-pre"></pre>
    </div>

    <div class="csr-card">
      <div class="csr-card-header">
        <h2>Private Key</h2>
        <div class="csr-out-actions">
          <button class="csr-copy-btn" data-target="csr-key-pre">Copy</button>
          <button class="csr-dl-btn" data-target="csr-key-pre" data-name="private.key">Download .key</button>
        </div>
      </div>
      <div class="csr-key-warn">
        <svg width="15" height="15" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg" style="flex-shrink:0;margin-top:1px"><path d="M10 2L2 17h16L10 2z" stroke="#f59e0b" stroke-width="1.5" stroke-linejoin="round" fill="none"/><line x1="10" y1="9" x2="10" y2="13" stroke="#f59e0b" stroke-width="1.5"/><circle cx="10" cy="15.5" r="0.75" fill="#f59e0b"/></svg>
        <span><strong>Keep this private key secure.</strong> Do not paste it into forms, commit it to git, or share it. Store it encrypted or in a secrets manager. This page does not retain it.</span>
      </div>
      <pre id="csr-key-pre" class="csr-pre"></pre>
    </div>

    <div class="csr-verify-block">
      <span class="csr-verify-label">Verify with OpenSSL</span>
      <pre class="csr-verify-pre">openssl req -verify -in request.csr -key private.key -noout
openssl req -text -noout -in request.csr</pre>
    </div>

  </div>
</div>

<style>
.tool-page-container { max-width: 900px; }
.page-hero { margin-bottom: 0.25rem; }
.page-hero h1 { color: var(--fg, #e2e8f0); font-size: 2rem; font-weight: 800; letter-spacing: -0.02em; margin-bottom: 0.35rem; }
.page-hero p  { color: var(--muted, #94a3b8); font-size: 0.95rem; }

.csr-app { display: flex; flex-direction: column; gap: 1.5rem; }

.csr-card { background: var(--card, #1a2233); border: 1px solid var(--border, #2d3a4f); border-radius: 10px; overflow: hidden; }

.csr-card-header { align-items: center; border-bottom: 1px solid var(--border, #2d3a4f); display: flex; gap: 1rem; justify-content: space-between; padding: 1rem 1.5rem; flex-wrap: wrap; }
.csr-card-header h2 { color: var(--fg, #e2e8f0); font-size: 1rem; font-weight: 700; margin: 0; }
.csr-header-hint { color: var(--muted, #94a3b8); font-size: 0.8rem; }

.csr-form-body { padding: 1.5rem; }
.csr-fields-grid { display: grid; gap: 1.1rem; grid-template-columns: 1fr 1fr; }
.csr-field-wide { grid-column: 1 / -1; }
.csr-field { display: flex; flex-direction: column; gap: 0.35rem; }
.csr-label { color: var(--muted, #94a3b8); font-size: 0.75rem; font-weight: 700; letter-spacing: 0.04em; }
.csr-req  { color: #f87171; }
.csr-input {
  background: #0d1420; border: 1px solid var(--border, #2d3a4f); border-radius: 7px;
  color: var(--fg, #e2e8f0); font-family: 'SF Mono','Fira Code',ui-monospace,monospace;
  font-size: 0.9rem; outline: none; padding: 0.55rem 0.9rem; transition: border-color 0.15s;
}
.csr-input:focus { border-color: var(--accent, #3b82f6); }
.csr-field-hint { color: var(--muted, #94a3b8); font-size: 0.72rem; }

.csr-san-body { padding: 1.25rem 1.5rem; }
.csr-textarea { background: #0d1420; border: 1px solid var(--border, #2d3a4f); border-radius: 7px; box-sizing: border-box; color: var(--fg, #e2e8f0); display: block; font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.85rem; line-height: 1.6; outline: none; padding: 0.65rem 0.9rem; resize: vertical; transition: border-color 0.15s; width: 100%; }
.csr-textarea:focus { border-color: var(--accent, #3b82f6); }
.csr-san-note { color: var(--muted, #94a3b8); font-size: 0.8rem; margin: 0.65rem 0 0; }

.csr-key-body { padding: 1.25rem 1.5rem; display: flex; flex-direction: column; gap: 0.75rem; }
.csr-key-opts { display: flex; flex-wrap: wrap; gap: 0.6rem; }
.csr-key-opt {
  background: transparent; border: 1px solid var(--border, #2d3a4f); border-radius: 8px;
  color: var(--muted, #94a3b8); cursor: pointer; font-size: 0.85rem; font-weight: 600;
  padding: 0.5rem 1.1rem; transition: color 0.15s, border-color 0.15s, background 0.15s;
  display: inline-flex; align-items: center; gap: 0.4rem;
}
.csr-key-opt:hover { color: var(--fg, #e2e8f0); border-color: var(--accent, #3b82f6); }
.csr-key-active { background: rgba(59,130,246,0.12); border-color: var(--accent, #3b82f6); color: var(--fg, #e2e8f0); }
.csr-rec-badge { background: rgba(16,185,129,0.15); border: 1px solid rgba(16,185,129,0.3); border-radius: 4px; color: #34d399; font-size: 0.65rem; font-weight: 700; padding: 0.1rem 0.35rem; }
.csr-slow-badge { background: rgba(245,158,11,0.12); border: 1px solid rgba(245,158,11,0.28); border-radius: 4px; color: #fbbf24; font-size: 0.65rem; font-weight: 700; padding: 0.1rem 0.35rem; }
.csr-key-note { color: var(--muted, #94a3b8); font-size: 0.82rem; margin: 0; }

.csr-error { background: rgba(239,68,68,0.08); border: 1px solid rgba(239,68,68,0.25); border-radius: 10px; color: #fca5a5; font-size: 0.875rem; padding: 1rem 1.25rem; }

.csr-gen-btn { background: var(--accent, #3b82f6); border: none; border-radius: 9px; color: #fff; cursor: pointer; font-size: 1rem; font-weight: 700; padding: 0.85rem 2rem; transition: background 0.15s; }
.csr-gen-btn:hover:not(:disabled) { background: #2563eb; }
.csr-gen-btn:disabled { opacity: 0.45; cursor: not-allowed; }

.csr-loading { align-items: center; background: var(--card, #1a2233); border: 1px solid var(--border, #2d3a4f); border-radius: 10px; color: var(--muted, #94a3b8); display: flex; font-size: 0.875rem; gap: 0.75rem; padding: 1.25rem 1.5rem; }
.csr-spinner { animation: csr-spin 0.7s linear infinite; border: 2px solid var(--border, #2d3a4f); border-radius: 50%; border-top-color: var(--accent, #3b82f6); flex-shrink: 0; height: 16px; width: 16px; }
@keyframes csr-spin { to { transform: rotate(360deg); } }

.csr-out-actions { display: flex; gap: 0.5rem; }
.csr-copy-btn { background: transparent; border: 1px solid var(--border, #2d3a4f); border-radius: 7px; color: var(--muted, #94a3b8); cursor: pointer; font-size: 0.78rem; font-weight: 600; padding: 0.3rem 0.8rem; transition: color 0.15s, border-color 0.15s; white-space: nowrap; }
.csr-copy-btn:hover { border-color: var(--accent, #3b82f6); color: var(--fg, #e2e8f0); }
.csr-dl-btn { background: transparent; border: 1px solid var(--border, #2d3a4f); border-radius: 7px; color: var(--muted, #94a3b8); cursor: pointer; font-size: 0.78rem; font-weight: 600; padding: 0.3rem 0.8rem; transition: color 0.15s, border-color 0.15s; white-space: nowrap; }
.csr-dl-btn:hover { border-color: var(--accent, #3b82f6); color: var(--fg, #e2e8f0); }

.csr-pre { background: #0d1420; border: none; color: var(--fg, #e2e8f0); font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.78rem; line-height: 1.6; margin: 0; overflow-x: auto; padding: 1.25rem 1.5rem; white-space: pre; word-break: break-all; }

.csr-key-warn { align-items: flex-start; background: rgba(245,158,11,0.07); border-bottom: 1px solid var(--border, #2d3a4f); color: #fcd34d; display: flex; font-size: 0.82rem; gap: 0.65rem; padding: 0.85rem 1.5rem; }
.csr-key-warn strong { color: #fbbf24; }

.csr-verify-block { background: var(--card, #1a2233); border: 1px solid var(--border, #2d3a4f); border-radius: 10px; overflow: hidden; }
.csr-verify-label { background: rgba(99,102,241,0.08); border-bottom: 1px solid var(--border, #2d3a4f); color: #a5b4fc; display: block; font-size: 0.72rem; font-weight: 700; letter-spacing: 0.08em; padding: 0.6rem 1.5rem; }
.csr-verify-pre { background: transparent; border: none; color: var(--muted, #94a3b8); font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.82rem; line-height: 1.7; margin: 0; padding: 1rem 1.5rem; white-space: pre; }

@media (max-width: 640px) {
  .page-hero h1 { font-size: 1.6rem; }
  .csr-fields-grid { grid-template-columns: 1fr; }
  .csr-field-wide { grid-column: 1; }
  .csr-out-actions { flex-wrap: wrap; }
}
</style>

<script>
(function () {
  'use strict';

  // ── DER encoder ───────────────────────────────────────────────────────────────

  function cat(arrays) {
    var total = 0, i;
    for (i = 0; i < arrays.length; i++) total += arrays[i].length;
    var out = new Uint8Array(total), off = 0;
    for (i = 0; i < arrays.length; i++) { out.set(arrays[i], off); off += arrays[i].length; }
    return out;
  }

  function tlv(tag, value) {
    var len = value.length, lenBytes;
    if (len < 128) lenBytes = new Uint8Array([len]);
    else if (len < 256) lenBytes = new Uint8Array([0x81, len]);
    else lenBytes = new Uint8Array([0x82, (len >> 8) & 0xff, len & 0xff]);
    var out = new Uint8Array(1 + lenBytes.length + len);
    out[0] = tag;
    out.set(lenBytes, 1);
    out.set(value, 1 + lenBytes.length);
    return out;
  }

  function encodeOID(str) {
    var parts = str.split('.').map(Number);
    var bytes = [(parts[0] * 40) + parts[1]], i, v, enc, j;
    for (i = 2; i < parts.length; i++) {
      v = parts[i]; enc = [];
      if (v === 0) { bytes.push(0); continue; }
      while (v > 0) { enc.unshift(v & 0x7f); v = v >>> 7; }
      for (j = 0; j < enc.length - 1; j++) enc[j] |= 0x80;
      for (j = 0; j < enc.length; j++) bytes.push(enc[j]);
    }
    return new Uint8Array(bytes);
  }

  var enc = new TextEncoder();
  function dSEQ(v)   { return tlv(0x30, v); }
  function dSET(v)   { return tlv(0x31, v); }
  function dINT(v)   { return tlv(0x02, v); }
  function dOID(s)   { return tlv(0x06, encodeOID(s)); }
  function dUTF8(s)  { return tlv(0x0c, enc.encode(s)); }
  function dIA5(s)   { return tlv(0x16, enc.encode(s)); }
  function dBIT(v)   { return tlv(0x03, cat([new Uint8Array([0]), v])); }
  function dOCT(v)   { return tlv(0x04, v); }
  function dNULL()   { return new Uint8Array([0x05, 0x00]); }
  function dCtx(v)   { return tlv(0xa0, v); }  // [0] constructed (IMPLICIT SET OF)

  // ── Subject name ─────────────────────────────────────────────────────────────

  function buildSubject(f) {
    var rdns = [];
    function rdn(oid, val) {
      if (!val || !val.trim()) return;
      var vBytes = oid === '1.2.840.113549.1.9.1' ? dIA5(val.trim()) : dUTF8(val.trim());
      rdns.push(dSET(dSEQ(cat([dOID(oid), vBytes]))));
    }
    rdn('2.5.4.6',  f.c);
    rdn('2.5.4.8',  f.st);
    rdn('2.5.4.7',  f.l);
    rdn('2.5.4.10', f.o);
    rdn('2.5.4.11', f.ou);
    rdn('2.5.4.3',  f.cn);
    return dSEQ(cat(rdns));
  }

  // ── SAN extension in [0] attributes ──────────────────────────────────────────

  var IP4_RE = /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/;

  function parseSANs(text) {
    return text.split('\n').map(function(l) { return l.trim(); }).filter(Boolean).map(function(v) {
      return IP4_RE.test(v) ? { type: 'ip', value: v } : { type: 'dns', value: v };
    });
  }

  function buildAttrs(sans) {
    if (!sans || !sans.length) return new Uint8Array([0xa0, 0x00]);
    var gnBytes = sans.map(function(s) {
      if (s.type === 'ip') {
        return tlv(0x87, new Uint8Array(s.value.split('.').map(Number)));
      }
      return tlv(0x82, enc.encode(s.value));
    });
    var sanExt = dSEQ(cat([dOID('2.5.29.17'), dOCT(dSEQ(cat(gnBytes)))]));
    var attr   = dSEQ(cat([dOID('1.2.840.113549.1.9.14'), dSET(dSEQ(sanExt))]));
    return dCtx(attr);
  }

  // ── ECDSA P1363 → DER ────────────────────────────────────────────────────────

  function p1363toDER(buf) {
    var raw = new Uint8Array(buf), half = raw.length >> 1;

    function encInt(bytes) {
      var s = 0;
      while (s < bytes.length - 1 && bytes[s] === 0) s++;
      bytes = bytes.slice(s);
      if (bytes[0] & 0x80) { var p = new Uint8Array(bytes.length + 1); p.set(bytes, 1); bytes = p; }
      return dINT(bytes);
    }

    return dSEQ(cat([encInt(raw.slice(0, half)), encInt(raw.slice(half))]));
  }

  // ── PEM formatter ────────────────────────────────────────────────────────────

  function toPEM(der, label) {
    var bin = '', chunk = 8192, i;
    for (i = 0; i < der.length; i += chunk) bin += String.fromCharCode.apply(null, der.slice(i, i + chunk));
    var b64 = btoa(bin), lines = [];
    for (i = 0; i < b64.length; i += 64) lines.push(b64.slice(i, i + 64));
    return '-----BEGIN ' + label + '-----\n' + lines.join('\n') + '\n-----END ' + label + '-----';
  }

  // ── Core CSR generation ───────────────────────────────────────────────────────

  async function generateCSR(fields, keyType, sans) {
    var keyPair, spki, sigAlg, sigBytes, isEC = keyType.startsWith('ec');

    if (keyType === 'rsa-2048' || keyType === 'rsa-4096') {
      var bits = keyType === 'rsa-4096' ? 4096 : 2048;
      keyPair = await crypto.subtle.generateKey(
        { name: 'RSASSA-PKCS1-v1_5', modulusLength: bits, publicExponent: new Uint8Array([1,0,1]), hash: 'SHA-256' },
        true, ['sign', 'verify']
      );
      spki = new Uint8Array(await crypto.subtle.exportKey('spki', keyPair.publicKey));
      var sigAlgOID  = '1.2.840.113549.1.1.11'; // sha256WithRSAEncryption
      var sigAlgBytes = dSEQ(cat([dOID(sigAlgOID), dNULL()]));

      var tbsInfo = buildTBS(fields, spki, sans);
      var rawSig  = new Uint8Array(await crypto.subtle.sign('RSASSA-PKCS1-v1_5', keyPair.privateKey, tbsInfo));
      var csr = dSEQ(cat([tbsInfo, sigAlgBytes, dBIT(rawSig)]));
      var pk8 = new Uint8Array(await crypto.subtle.exportKey('pkcs8', keyPair.privateKey));
      return { csr: toPEM(csr, 'CERTIFICATE REQUEST'), key: toPEM(pk8, 'PRIVATE KEY') };
    }

    var curve   = keyType === 'ec-p256' ? 'P-256' : 'P-384';
    var hashAlg = keyType === 'ec-p256' ? 'SHA-256' : 'SHA-384';
    var ecSigOID = keyType === 'ec-p256' ? '1.2.840.10045.4.3.2' : '1.2.840.10045.4.3.3';
    keyPair = await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: curve }, true, ['sign', 'verify']);
    spki = new Uint8Array(await crypto.subtle.exportKey('spki', keyPair.publicKey));
    var ecSigAlg = dSEQ(dOID(ecSigOID));

    var tbsInfo = buildTBS(fields, spki, sans);
    var rawSig  = await crypto.subtle.sign({ name: 'ECDSA', hash: hashAlg }, keyPair.privateKey, tbsInfo);
    var derSig  = p1363toDER(rawSig);
    var csr = dSEQ(cat([tbsInfo, ecSigAlg, dBIT(derSig)]));
    var pk8 = new Uint8Array(await crypto.subtle.exportKey('pkcs8', keyPair.privateKey));
    return { csr: toPEM(csr, 'CERTIFICATE REQUEST'), key: toPEM(pk8, 'PRIVATE KEY') };
  }

  function buildTBS(fields, spki, sans) {
    var version = dINT(new Uint8Array([0]));
    var subject = buildSubject(fields);
    var attrs   = buildAttrs(sans);
    return dSEQ(cat([version, subject, spki, attrs]));
  }

  // ── UI ────────────────────────────────────────────────────────────────────────

  var selectedKey = 'ec-p256';
  var KEY_NOTES = {
    'ec-p256':  'P-256: 128-bit security, fastest signing, smallest signatures. Supported by all modern TLS stacks.',
    'ec-p384':  'P-384: 192-bit security, NSA Suite B compliant. Required by some compliance and government standards.',
    'rsa-2048': 'RSA 2048: 112-bit security. Broadly compatible, including legacy systems. CA/B Forum minimum.',
    'rsa-4096': 'RSA 4096: 140-bit security. Key generation takes several seconds in-browser. Use only if required by policy.',
  };

  document.getElementById('csr-key-opts').addEventListener('click', function(e) {
    var btn = e.target.closest('[data-val]');
    if (!btn) return;
    document.querySelectorAll('.csr-key-opt').forEach(function(b) { b.classList.remove('csr-key-active'); });
    btn.classList.add('csr-key-active');
    selectedKey = btn.dataset.val;
    document.getElementById('csr-key-note').textContent = KEY_NOTES[selectedKey];
  });

  document.getElementById('csr-gen-btn').addEventListener('click', async function() {
    var genBtn  = document.getElementById('csr-gen-btn');
    var errorEl = document.getElementById('csr-error');
    var loading = document.getElementById('csr-loading');
    var output  = document.getElementById('csr-output');
    var msgEl   = document.getElementById('csr-loading-msg');

    var cn = document.getElementById('csr-cn').value.trim();
    var c  = document.getElementById('csr-c').value.trim().toUpperCase();

    errorEl.style.display = 'none';
    output.style.display  = 'none';

    if (!cn) {
      errorEl.textContent  = '⚠ Common Name (CN) is required.';
      errorEl.style.display = '';
      return;
    }
    if (c && c.length !== 2) {
      errorEl.textContent  = '⚠ Country must be a 2-letter ISO code (e.g. US, GB).';
      errorEl.style.display = '';
      return;
    }

    var fields = {
      cn: cn,
      o:  document.getElementById('csr-o').value.trim(),
      ou: document.getElementById('csr-ou').value.trim(),
      l:  document.getElementById('csr-l').value.trim(),
      st: document.getElementById('csr-st').value.trim(),
      c:  c,
    };

    var sans = parseSANs(document.getElementById('csr-sans').value);

    genBtn.disabled = true;
    loading.style.display = '';
    msgEl.textContent = selectedKey.startsWith('rsa-4096') ? 'Generating RSA 4096 key — this may take a few seconds…' : 'Generating key pair…';

    try {
      var result = await generateCSR(fields, selectedKey, sans);
      document.getElementById('csr-csr-pre').textContent = result.csr;
      document.getElementById('csr-key-pre').textContent = result.key;
      output.style.display = '';
      output.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    } catch (e) {
      errorEl.textContent  = '⚠ ' + (e.message || 'Key generation failed — check browser console for details.');
      errorEl.style.display = '';
    } finally {
      genBtn.disabled = false;
      loading.style.display = 'none';
    }
  });

  // Copy buttons
  document.addEventListener('click', function(e) {
    var btn = e.target.closest('.csr-copy-btn');
    if (!btn) return;
    var pre = document.getElementById(btn.dataset.target);
    if (!pre) return;
    navigator.clipboard.writeText(pre.textContent).then(function() {
      var orig = btn.textContent;
      btn.textContent = 'Copied!';
      setTimeout(function() { btn.textContent = orig; }, 1500);
    });
  });

  // Download buttons
  document.addEventListener('click', function(e) {
    var btn = e.target.closest('.csr-dl-btn');
    if (!btn) return;
    var pre = document.getElementById(btn.dataset.target);
    if (!pre) return;
    var blob = new Blob([pre.textContent], { type: 'text/plain' });
    var url  = URL.createObjectURL(blob);
    var a    = document.createElement('a');
    a.href = url; a.download = btn.dataset.name;
    document.body.appendChild(a); a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  });
})();
</script>
