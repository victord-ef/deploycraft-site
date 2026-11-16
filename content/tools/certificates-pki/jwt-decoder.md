---
title: "JWT Decoder"
description: "Decode and inspect JSON Web Tokens — header, payload, claims, expiry status, and signature algorithm. No data sent to any server."
status: "live"
icon: "🪙"
tags: ["jwt", "json-web-token", "authentication", "oauth", "oidc", "pki", "decoder"]
weight: 4
---

<div class="jwt-app">

  <div class="page-hero">
    <h1>JWT Decoder</h1>
    <p>Paste any JSON Web Token to inspect its header, claims, and expiry status — decoded entirely in your browser, nothing sent anywhere.</p>
  </div>

  <!-- Input -->
  <div class="jwt-card">
    <div class="jwt-card-header">
      <h2>Token Input</h2>
      <div class="jwt-header-actions">
        <button id="jwt-example-btn" class="jwt-btn-secondary">Load example</button>
        <button id="jwt-clear-btn" class="jwt-btn-ghost">Clear</button>
      </div>
    </div>
    <div class="jwt-input-body">
      <textarea id="jwt-input" class="jwt-textarea" spellcheck="false" autocomplete="off" rows="5"
        placeholder="eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.signature"></textarea>
      <div class="jwt-input-footer">
        <span class="jwt-hint">Paste a JWT (HS256, RS256, ES256, etc.) — JWE encrypted tokens are not supported</span>
        <button id="jwt-decode-btn" class="jwt-btn-primary">Decode →</button>
      </div>
    </div>
  </div>

  <!-- Error -->
  <div id="jwt-error" class="jwt-error" style="display:none"></div>

  <!-- Results -->
  <div id="jwt-result" style="display:none">

    <!-- Token structure visualiser -->
    <div class="jwt-card">
      <div class="jwt-card-header"><h2>Token Structure</h2></div>
      <div class="jwt-structure-body">
        <div class="jwt-parts-row">
          <div class="jwt-part jwt-part-header">
            <span class="jwt-part-label">HEADER</span>
            <span id="jwt-part-h" class="jwt-part-value"></span>
          </div>
          <span class="jwt-dot">.</span>
          <div class="jwt-part jwt-part-payload">
            <span class="jwt-part-label">PAYLOAD</span>
            <span id="jwt-part-p" class="jwt-part-value"></span>
          </div>
          <span class="jwt-dot">.</span>
          <div class="jwt-part jwt-part-sig">
            <span class="jwt-part-label">SIGNATURE</span>
            <span id="jwt-part-s" class="jwt-part-value"></span>
          </div>
        </div>
      </div>
    </div>

    <!-- Header -->
    <div class="jwt-two-col">
      <div class="jwt-card">
        <div class="jwt-card-header"><h2>Header</h2></div>
        <table class="jwt-table">
          <tr><th>Algorithm</th><td id="jwt-alg"></td></tr>
          <tr><th>Type</th><td id="jwt-typ"></td></tr>
          <tr><th>Key ID</th><td id="jwt-kid"></td></tr>
          <tr><th>Content Type</th><td id="jwt-cty"></td></tr>
        </table>
        <div class="jwt-json-block">
          <span class="jwt-json-label">RAW</span>
          <pre id="jwt-header-json" class="jwt-json-pre"></pre>
        </div>
      </div>

      <!-- Timing -->
      <div class="jwt-card">
        <div class="jwt-card-header">
          <h2>Timing</h2>
          <div id="jwt-timing-badge" class="jwt-timing-badge" style="display:none"></div>
        </div>
        <table class="jwt-table">
          <tr><th>Issued At</th><td id="jwt-iat"></td></tr>
          <tr><th>Expires</th><td id="jwt-exp"></td></tr>
          <tr><th>Not Before</th><td id="jwt-nbf"></td></tr>
          <tr><th>Time Left</th><td id="jwt-ttl"></td></tr>
        </table>
      </div>
    </div>

    <!-- Payload -->
    <div class="jwt-card">
      <div class="jwt-card-header"><h2>Payload Claims</h2></div>
      <div id="jwt-claims-table-wrap">
        <table class="jwt-table jwt-claims-table">
          <thead><tr><th class="jwt-th-claim">Claim</th><th class="jwt-th-name">Name</th><th class="jwt-th-val">Value</th></tr></thead>
          <tbody id="jwt-claims-body"></tbody>
        </table>
      </div>
      <div class="jwt-json-block">
        <span class="jwt-json-label">RAW</span>
        <pre id="jwt-payload-json" class="jwt-json-pre"></pre>
      </div>
    </div>

    <!-- Signature note -->
    <div class="jwt-card">
      <div class="jwt-card-header"><h2>Signature</h2></div>
      <div class="jwt-sig-body">
        <div id="jwt-sig-note" class="jwt-sig-note"></div>
        <table class="jwt-table">
          <tr><th>Algorithm</th><td id="jwt-sig-alg"></td></tr>
          <tr><th>Base64url</th><td id="jwt-sig-b64" class="jwt-mono-wrap"></td></tr>
        </table>
      </div>
    </div>

  </div>
</div>

<style>
.tool-page-container { max-width: 920px; }
.page-hero { margin-bottom: 0.25rem; }
.page-hero h1 { color: var(--fg, #e2e8f0); font-size: 2rem; font-weight: 800; letter-spacing: -0.02em; margin-bottom: 0.35rem; }
.page-hero p  { color: var(--muted, #94a3b8); font-size: 0.95rem; }

.jwt-app { display: flex; flex-direction: column; gap: 1.5rem; }

.jwt-card { background: var(--card, #1a2233); border: 1px solid var(--border, #2d3a4f); border-radius: 10px; overflow: hidden; }

.jwt-card-header { align-items: center; border-bottom: 1px solid var(--border, #2d3a4f); display: flex; gap: 1rem; justify-content: space-between; padding: 1rem 1.5rem; flex-wrap: wrap; }
.jwt-card-header h2 { color: var(--fg, #e2e8f0); font-size: 1rem; font-weight: 700; margin: 0; }
.jwt-header-actions { display: flex; gap: 0.5rem; }

.jwt-btn-primary { background: var(--accent, #3b82f6); border: none; border-radius: 7px; color: #fff; cursor: pointer; font-size: 0.875rem; font-weight: 700; padding: 0.55rem 1.25rem; transition: background 0.15s; white-space: nowrap; }
.jwt-btn-primary:hover { background: #2563eb; }

.jwt-btn-secondary { background: transparent; border: 1px solid var(--border, #2d3a4f); border-radius: 7px; color: var(--muted, #94a3b8); cursor: pointer; font-size: 0.8rem; font-weight: 600; padding: 0.4rem 0.9rem; transition: color 0.15s, border-color 0.15s; white-space: nowrap; }
.jwt-btn-secondary:hover { color: var(--fg, #e2e8f0); border-color: var(--accent, #3b82f6); }

.jwt-btn-ghost { background: transparent; border: none; color: var(--muted, #94a3b8); cursor: pointer; font-size: 0.8rem; padding: 0.4rem 0.5rem; transition: color 0.15s; }
.jwt-btn-ghost:hover { color: var(--fg, #e2e8f0); }

.jwt-input-body { padding: 1.25rem 1.5rem; }
.jwt-textarea { background: #0d1420; border: 1px solid var(--border, #2d3a4f); border-radius: 8px; box-sizing: border-box; color: var(--fg, #e2e8f0); display: block; font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.82rem; line-height: 1.6; outline: none; padding: 0.75rem 1rem; resize: vertical; transition: border-color 0.15s; width: 100%; }
.jwt-textarea:focus { border-color: var(--accent, #3b82f6); }
.jwt-input-footer { align-items: center; display: flex; gap: 1rem; justify-content: space-between; margin-top: 0.75rem; flex-wrap: wrap; }
.jwt-hint { color: var(--muted, #94a3b8); font-size: 0.78rem; }

.jwt-error { background: rgba(239,68,68,0.08); border: 1px solid rgba(239,68,68,0.25); border-radius: 10px; color: #fca5a5; font-size: 0.875rem; padding: 1rem 1.25rem; }

/* Token structure */
.jwt-structure-body { padding: 1.25rem 1.5rem; overflow-x: auto; }
.jwt-parts-row { display: flex; align-items: flex-start; gap: 0; min-width: 0; }
.jwt-part { display: flex; flex-direction: column; gap: 0.3rem; min-width: 0; flex: 1; }
.jwt-part-label { font-size: 0.65rem; font-weight: 700; letter-spacing: 0.08em; }
.jwt-part-header .jwt-part-label { color: #f87171; }
.jwt-part-payload .jwt-part-label { color: #a78bfa; }
.jwt-part-sig    .jwt-part-label  { color: #34d399; }
.jwt-part-value { background: #0d1420; border-radius: 6px; font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.72rem; overflow: hidden; padding: 0.4rem 0.6rem; text-overflow: ellipsis; white-space: nowrap; word-break: break-all; }
.jwt-part-header .jwt-part-value { border: 1px solid rgba(248,113,113,0.3); color: #fca5a5; }
.jwt-part-payload .jwt-part-value { border: 1px solid rgba(167,139,250,0.3); color: #c4b5fd; }
.jwt-part-sig    .jwt-part-value  { border: 1px solid rgba(52,211,153,0.3); color: #6ee7b7; }
.jwt-dot { color: var(--muted, #94a3b8); font-size: 1.5rem; font-weight: 700; line-height: 1; padding: 1.5rem 0.25rem 0; flex-shrink: 0; }

/* Two column layout */
.jwt-two-col { display: grid; gap: 1.5rem; grid-template-columns: 1fr 1fr; }

/* Table */
.jwt-table { border-collapse: collapse; font-size: 0.85rem; width: 100%; }
.jwt-table th { border-bottom: 1px solid var(--border, #2d3a4f); color: var(--muted, #94a3b8); font-size: 0.72rem; font-weight: 700; letter-spacing: 0.06em; padding: 0.65rem 1.5rem; text-align: left; vertical-align: top; white-space: nowrap; width: 110px; }
.jwt-table td { border-bottom: 1px solid var(--border, #2d3a4f); color: var(--fg, #e2e8f0); font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.82rem; padding: 0.65rem 1.5rem 0.65rem 0; vertical-align: top; word-break: break-all; }
.jwt-table tr:last-child th, .jwt-table tr:last-child td { border-bottom: none; }
.jwt-mono-wrap { word-break: break-all; }

/* Claims table */
.jwt-claims-table { table-layout: fixed; }
.jwt-th-claim { width: 100px; }
.jwt-th-name  { width: 150px; }
.jwt-th-val   {}
.jwt-claims-table th { background: rgba(0,0,0,0.15); border-bottom: 1px solid var(--border, #2d3a4f); color: var(--muted, #94a3b8); font-size: 0.7rem; font-weight: 700; letter-spacing: 0.06em; padding: 0.5rem 1.5rem; text-align: left; }
.jwt-claim-code { color: #93c5fd; font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.8rem; }
.jwt-claim-name { color: var(--muted, #94a3b8); font-size: 0.78rem; }
.jwt-claim-val  { color: var(--fg, #e2e8f0); font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.8rem; word-break: break-all; }
.jwt-claim-val.jwt-val-timestamp { color: #a78bfa; }
.jwt-claim-val.jwt-val-bool-true  { color: #34d399; }
.jwt-claim-val.jwt-val-bool-false { color: #f87171; }
.jwt-claim-val.jwt-val-number { color: #fbbf24; }
.jwt-claim-val.jwt-val-array, .jwt-claim-val.jwt-val-object { color: #94a3b8; }

/* JSON raw block */
.jwt-json-block { border-top: 1px solid var(--border, #2d3a4f); }
.jwt-json-label { background: rgba(0,0,0,0.15); border-bottom: 1px solid var(--border, #2d3a4f); color: var(--muted, #94a3b8); display: block; font-size: 0.65rem; font-weight: 700; letter-spacing: 0.08em; padding: 0.4rem 1.5rem; }
.jwt-json-pre { background: transparent; border: none; color: var(--fg, #e2e8f0); font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.8rem; line-height: 1.65; margin: 0; overflow-x: auto; padding: 1rem 1.5rem; white-space: pre; }

/* Timing badge */
.jwt-timing-badge { border-radius: 5px; font-size: 0.72rem; font-weight: 700; padding: 0.25rem 0.65rem; white-space: nowrap; }
.jwt-timing-badge.expired  { background: rgba(239,68,68,0.1);  border: 1px solid rgba(239,68,68,0.3);  color: #f87171; }
.jwt-timing-badge.valid    { background: rgba(16,185,129,0.1); border: 1px solid rgba(16,185,129,0.3); color: #34d399; }
.jwt-timing-badge.no-exp   { background: rgba(148,163,184,0.1); border: 1px solid rgba(148,163,184,0.2); color: #94a3b8; }
.jwt-timing-badge.not-yet  { background: rgba(245,158,11,0.1); border: 1px solid rgba(245,158,11,0.3); color: #fbbf24; }

/* Signature */
.jwt-sig-body { display: flex; flex-direction: column; }
.jwt-sig-note { border-bottom: 1px solid var(--border, #2d3a4f); font-size: 0.82rem; padding: 0.85rem 1.5rem; }
.jwt-sig-note.unsigned { color: var(--muted, #94a3b8); }
.jwt-sig-note.unverified { background: rgba(245,158,11,0.06); color: #fcd34d; }

@media (max-width: 680px) {
  .page-hero h1 { font-size: 1.6rem; }
  .jwt-two-col { grid-template-columns: 1fr; }
  .jwt-table th { padding: 0.55rem 1rem; width: 90px; }
  .jwt-table td { padding: 0.55rem 0.75rem 0.55rem 0; }
}
</style>

<script>
(function () {
  'use strict';

  // ── Known claim registry ──────────────────────────────────────────────────────
  var CLAIM_NAMES = {
    iss: 'Issuer',         sub: 'Subject',         aud: 'Audience',
    exp: 'Expiry',         nbf: 'Not Before',       iat: 'Issued At',
    jti: 'JWT ID',         azp: 'Authorized Party', scope: 'Scope',
    sid: 'Session ID',     nonce: 'Nonce',          at_hash: 'Access Token Hash',
    c_hash: 'Code Hash',   acr: 'Auth Context Ref', amr: 'Auth Methods',
    auth_time: 'Auth Time',
    name: 'Full Name',     given_name: 'Given Name', family_name: 'Family Name',
    middle_name: 'Middle Name', nickname: 'Nickname', preferred_username: 'Preferred Username',
    profile: 'Profile URL', picture: 'Picture URL', website: 'Website',
    email: 'Email',        email_verified: 'Email Verified',
    gender: 'Gender',      birthdate: 'Birthdate',  zoneinfo: 'Timezone',
    locale: 'Locale',      phone_number: 'Phone',   phone_number_verified: 'Phone Verified',
    address: 'Address',    updated_at: 'Updated At',
    roles: 'Roles',        groups: 'Groups',        permissions: 'Permissions',
    tenant_id: 'Tenant ID', org_id: 'Org ID',       client_id: 'Client ID',
    ver: 'Version',        tid: 'Tenant ID (MS)',    oid: 'Object ID (MS)',
    upn: 'UPN (MS)',       unique_name: 'Unique Name (MS)',
    appid: 'App ID (MS)',  appidacr: 'App ID ACR (MS)',
    scp: 'Scope (MS)',     wids: 'Directory Roles (MS)',
  };

  var TIMESTAMP_CLAIMS = new Set(['exp', 'iat', 'nbf', 'auth_time', 'updated_at']);

  var ALG_NOTES = {
    HS256: 'HMAC-SHA256 — symmetric, shared secret',
    HS384: 'HMAC-SHA384 — symmetric, shared secret',
    HS512: 'HMAC-SHA512 — symmetric, shared secret',
    RS256: 'RSA-PKCS1v1.5 with SHA-256 — asymmetric',
    RS384: 'RSA-PKCS1v1.5 with SHA-384 — asymmetric',
    RS512: 'RSA-PKCS1v1.5 with SHA-512 — asymmetric',
    ES256: 'ECDSA P-256 with SHA-256 — asymmetric',
    ES384: 'ECDSA P-384 with SHA-384 — asymmetric',
    ES512: 'ECDSA P-521 with SHA-512 — asymmetric',
    PS256: 'RSA-PSS with SHA-256 — asymmetric',
    PS384: 'RSA-PSS with SHA-384 — asymmetric',
    PS512: 'RSA-PSS with SHA-512 — asymmetric',
    EdDSA: 'EdDSA (Ed25519 / Ed448) — asymmetric',
    none:  'Unsecured JWT — no signature verification',
  };

  // ── Decode helpers ────────────────────────────────────────────────────────────

  function b64urlDecode(str) {
    // Pad and convert base64url to base64
    str = str.replace(/-/g, '+').replace(/_/g, '/');
    while (str.length % 4) str += '=';
    return atob(str);
  }

  function b64urlToJSON(str) {
    return JSON.parse(b64urlDecode(str));
  }

  function formatTS(unix) {
    if (!unix) return null;
    var d = new Date(unix * 1000);
    return d.toUTCString().replace(' GMT', ' UTC') + '  (' + unix + ')';
  }

  function decodeJWT(token) {
    token = token.trim();
    var parts = token.split('.');
    if (parts.length < 2 || parts.length > 3) throw new Error('A JWT must have exactly 3 dot-separated parts.');

    var header, payload;
    try { header = b64urlToJSON(parts[0]); }
    catch (e) { throw new Error('Header is not valid base64url-encoded JSON: ' + e.message); }

    try { payload = b64urlToJSON(parts[1]); }
    catch (e) { throw new Error('Payload is not valid base64url-encoded JSON: ' + e.message); }

    if (header.enc) throw new Error('This appears to be a JWE (encrypted token). JWE decryption requires the private key and is not supported.');

    return {
      parts:     parts,
      headerRaw: parts[0],
      payloadRaw: parts[1],
      sigRaw:    parts[2] || '',
      header:    header,
      payload:   payload,
    };
  }

  // ── Render helpers ────────────────────────────────────────────────────────────

  function esc(s) {
    return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
  }

  function set(id, val, fallback) {
    var el = document.getElementById(id);
    if (el) el.textContent = (val !== null && val !== undefined && val !== '') ? val : (fallback || '—');
  }

  function truncate(s, n) {
    return s.length > n ? s.slice(0, n) + '…' : s;
  }

  function prettyJSON(obj) {
    return JSON.stringify(obj, null, 2);
  }

  function renderClaimValue(key, val) {
    if (TIMESTAMP_CLAIMS.has(key) && typeof val === 'number') {
      var d = new Date(val * 1000);
      return { text: d.toUTCString().replace(' GMT', ' UTC') + '  (' + val + ')', cls: 'jwt-val-timestamp' };
    }
    if (typeof val === 'boolean') {
      return { text: String(val), cls: val ? 'jwt-val-bool-true' : 'jwt-val-bool-false' };
    }
    if (typeof val === 'number') {
      return { text: String(val), cls: 'jwt-val-number' };
    }
    if (Array.isArray(val)) {
      return { text: JSON.stringify(val), cls: 'jwt-val-array' };
    }
    if (val !== null && typeof val === 'object') {
      return { text: JSON.stringify(val, null, 2), cls: 'jwt-val-object' };
    }
    return { text: String(val), cls: '' };
  }

  function render(decoded) {
    var h = decoded.header, p = decoded.payload, now = Math.floor(Date.now() / 1000);

    // Structure visualiser
    document.getElementById('jwt-part-h').textContent = truncate(decoded.headerRaw, 48);
    document.getElementById('jwt-part-p').textContent = truncate(decoded.payloadRaw, 60);
    document.getElementById('jwt-part-s').textContent = decoded.sigRaw ? truncate(decoded.sigRaw, 40) : '(none)';

    // Header fields
    set('jwt-alg', h.alg ? h.alg + (ALG_NOTES[h.alg] ? '  —  ' + ALG_NOTES[h.alg] : '') : null);
    set('jwt-typ', h.typ);
    set('jwt-kid', h.kid);
    set('jwt-cty', h.cty);
    document.getElementById('jwt-header-json').textContent = prettyJSON(h);

    // Timing
    var exp = p.exp, iat = p.iat, nbf = p.nbf;
    var badgeEl = document.getElementById('jwt-timing-badge');

    if (exp) {
      var secsLeft = exp - now;
      var daysLeft = Math.floor(secsLeft / 86400);
      if (secsLeft < 0) {
        badgeEl.textContent = 'Expired ' + Math.abs(daysLeft) + 'd ago';
        badgeEl.className = 'jwt-timing-badge expired';
      } else {
        var label = daysLeft > 0 ? daysLeft + 'd remaining' : Math.floor(secsLeft / 60) + 'm remaining';
        badgeEl.textContent = label;
        badgeEl.className = 'jwt-timing-badge valid';
      }
      badgeEl.style.display = '';
    } else if (nbf && nbf > now) {
      badgeEl.textContent = 'Not valid yet';
      badgeEl.className = 'jwt-timing-badge not-yet';
      badgeEl.style.display = '';
    } else {
      badgeEl.textContent = 'No expiry set';
      badgeEl.className = 'jwt-timing-badge no-exp';
      badgeEl.style.display = '';
    }

    set('jwt-iat', iat ? formatTS(iat) : null);
    set('jwt-exp', exp ? formatTS(exp) : null);
    set('jwt-nbf', nbf ? formatTS(nbf) : null);

    if (exp) {
      var secsLeft2 = exp - now;
      if (secsLeft2 < 0) {
        document.getElementById('jwt-ttl').textContent = 'Expired';
        document.getElementById('jwt-ttl').style.color = '#f87171';
      } else {
        var days2 = Math.floor(secsLeft2 / 86400);
        var hrs   = Math.floor((secsLeft2 % 86400) / 3600);
        var mins  = Math.floor((secsLeft2 % 3600) / 60);
        var ttl = days2 > 0 ? days2 + 'd ' + hrs + 'h' : hrs > 0 ? hrs + 'h ' + mins + 'm' : mins + 'm';
        document.getElementById('jwt-ttl').textContent = ttl;
        document.getElementById('jwt-ttl').style.color = '';
      }
    } else {
      set('jwt-ttl', null);
    }

    // Claims table
    var tbody = document.getElementById('jwt-claims-body');
    tbody.innerHTML = '';
    Object.keys(p).forEach(function(key) {
      var cv   = renderClaimValue(key, p[key]);
      var name = CLAIM_NAMES[key] || '';
      var tr   = document.createElement('tr');
      tr.innerHTML =
        '<td class="jwt-table-td"><span class="jwt-claim-code">' + esc(key) + '</span></td>' +
        '<td class="jwt-table-td"><span class="jwt-claim-name">' + esc(name) + '</span></td>' +
        '<td class="jwt-table-td"><span class="jwt-claim-val' + (cv.cls ? ' ' + esc(cv.cls) : '') + '">' + esc(cv.text) + '</span></td>';
      tbody.appendChild(tr);
    });

    document.getElementById('jwt-payload-json').textContent = prettyJSON(p);

    // Signature
    var alg = h.alg || 'unknown';
    var sigNoteEl = document.getElementById('jwt-sig-note');
    if (!decoded.sigRaw || alg === 'none') {
      sigNoteEl.textContent = 'This token has no signature — it is an unsecured JWT (alg: none). Do not trust claims without signature verification.';
      sigNoteEl.className = 'jwt-sig-note unverified';
    } else if (alg.startsWith('HS')) {
      sigNoteEl.textContent = 'HMAC signature — verification requires the shared secret. The signature is displayed below but cannot be verified in-browser without the secret.';
      sigNoteEl.className = 'jwt-sig-note unverified';
    } else {
      sigNoteEl.textContent = 'Asymmetric signature — verification requires the public key (JWK/JWKS). Structure is decoded but the signature is not verified here.';
      sigNoteEl.className = 'jwt-sig-note unverified';
    }

    set('jwt-sig-alg', alg + (ALG_NOTES[alg] ? '  —  ' + ALG_NOTES[alg] : ''));
    set('jwt-sig-b64', decoded.sigRaw || null);
  }

  // ── Entry point ───────────────────────────────────────────────────────────────

  function decode() {
    var token   = document.getElementById('jwt-input').value.trim();
    var errorEl = document.getElementById('jwt-error');
    var resultEl= document.getElementById('jwt-result');
    errorEl.style.display  = 'none';
    resultEl.style.display = 'none';
    if (!token) return;
    try {
      var decoded = decodeJWT(token);
      render(decoded);
      resultEl.style.display = '';
    } catch (e) {
      errorEl.textContent   = '⚠ ' + (e.message || 'Failed to decode token.');
      errorEl.style.display = '';
    }
  }

  // ── Example JWT (RS256, some standard claims) ─────────────────────────────────
  // Header: {"alg":"RS256","typ":"JWT","kid":"key-2026-01"}
  // Payload: standard OIDC-ish claims with exp in 2027
  var EXAMPLE = 'eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImtleS0yMDI2LTAxIn0.' +
    'eyJpc3MiOiJodHRwczovL2F1dGguZXhhbXBsZS5jb20iLCJzdWIiOiJ1c3JfN2sycXh6OW0iLCJhdWQiOlsiYXBpLmV4YW1wbGUuY29tIl0sImV4cCI6MTc3MjA2MDgwMCwibmJmIjoxNzMxNjI0NDAwLCJpYXQiOjE3MzE2MjQ0MDAsImp0aSI6ImUzYjBjNDQyOThmYyIsImVtYWlsIjoiamFuZS5kb2VAZXhhbXBsZS5jb20iLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwibmFtZSI6IkphbmUgRG9lIiwicHJlZmVycmVkX3VzZXJuYW1lIjoiamFuZS5kb2UiLCJyb2xlcyI6WyJlbmdpbmVlciIsImRlcGxveWVyIl0sInNjb3BlIjoib3BlbmlkIHByb2ZpbGUgZW1haWwgYXBpOnJlYWQifQ.' +
    'SIGNATURE_NOT_VERIFIED';

  // ── Events ────────────────────────────────────────────────────────────────────

  document.getElementById('jwt-decode-btn').addEventListener('click', decode);
  document.getElementById('jwt-example-btn').addEventListener('click', function () {
    document.getElementById('jwt-input').value = EXAMPLE;
    decode();
  });
  document.getElementById('jwt-clear-btn').addEventListener('click', function () {
    document.getElementById('jwt-input').value = '';
    document.getElementById('jwt-error').style.display  = 'none';
    document.getElementById('jwt-result').style.display = 'none';
  });
  document.getElementById('jwt-input').addEventListener('paste', function () {
    setTimeout(decode, 60);
  });
  document.getElementById('jwt-input').addEventListener('input', function () {
    var v = this.value.trim();
    if (v.split('.').length === 3) decode();
  });
})();
</script>
