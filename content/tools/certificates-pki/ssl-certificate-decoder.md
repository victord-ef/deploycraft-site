---
title: "SSL Certificate Decoder"
description: "Paste any PEM certificate to decode subject, issuer, SANs, validity, key type, and fingerprints — entirely in your browser, no data sent anywhere."
status: "live"
icon: "🔐"
tags: ["ssl", "tls", "certificates", "x509", "pem", "pki", "decoder"]
weight: 1
---

<div class="cert-app">

  <div class="page-hero">
    <h1>SSL Certificate Decoder</h1>
    <p>Paste a PEM-encoded X.509 certificate to inspect every field — decoded entirely in your browser, no data sent to any server.</p>
  </div>

  <div class="cert-card">
    <div class="cert-card-header">
      <h2>Certificate Input</h2>
      <div class="cert-header-actions">
        <button id="cert-example-btn" class="cert-btn-secondary">Load example</button>
        <button id="cert-clear-btn" class="cert-btn-ghost">Clear</button>
      </div>
    </div>
    <div class="cert-input-body">
      <textarea id="cert-input" class="cert-textarea" spellcheck="false" autocomplete="off" rows="8" placeholder="-----BEGIN CERTIFICATE-----&#10;MIIFazCCA1Og...&#10;-----END CERTIFICATE-----"></textarea>
      <div class="cert-input-footer">
        <span class="cert-hint">Paste a PEM certificate or drag &amp; drop a .pem / .crt file</span>
        <button id="cert-decode-btn" class="cert-btn-primary">Decode →</button>
      </div>
    </div>
  </div>

  <div id="cert-error" class="cert-error" style="display:none"></div>

  <div id="cert-result" style="display:none">

    <div class="cert-card">
      <div class="cert-card-header">
        <h2>Subject</h2>
        <div id="cert-self-signed" class="cert-badge cert-badge-warning" style="display:none">Self-signed</div>
      </div>
      <table class="cert-table">
        <tr><th>Common Name</th><td id="cert-subject-cn"></td></tr>
        <tr><th>Organization</th><td id="cert-subject-o"></td></tr>
        <tr><th>Org. Unit</th><td id="cert-subject-ou"></td></tr>
        <tr><th>Locality</th><td id="cert-subject-l"></td></tr>
        <tr><th>State / Province</th><td id="cert-subject-st"></td></tr>
        <tr><th>Country</th><td id="cert-subject-c"></td></tr>
      </table>
    </div>

    <div class="cert-card">
      <div class="cert-card-header"><h2>Subject Alternative Names</h2></div>
      <div class="cert-san-body"><div id="cert-sans" class="cert-san-list"></div></div>
    </div>

    <div class="cert-card">
      <div class="cert-card-header"><h2>Issuer</h2></div>
      <table class="cert-table">
        <tr><th>Common Name</th><td id="cert-issuer-cn"></td></tr>
        <tr><th>Organization</th><td id="cert-issuer-o"></td></tr>
        <tr><th>Country</th><td id="cert-issuer-c"></td></tr>
      </table>
    </div>

    <div class="cert-card">
      <div class="cert-card-header">
        <h2>Validity</h2>
        <div id="cert-days-badge" class="cert-days-badge"></div>
      </div>
      <table class="cert-table">
        <tr><th>Not Before</th><td id="cert-not-before"></td></tr>
        <tr><th>Not After</th><td id="cert-not-after"></td></tr>
      </table>
    </div>

    <div class="cert-card">
      <div class="cert-card-header"><h2>Technical Details</h2></div>
      <table class="cert-table">
        <tr><th>Version</th><td id="cert-version"></td></tr>
        <tr><th>Serial Number</th><td id="cert-serial" class="cert-mono-wrap"></td></tr>
        <tr><th>Signature Algorithm</th><td id="cert-sig-alg"></td></tr>
        <tr><th>Public Key</th><td id="cert-key-info"></td></tr>
        <tr><th>Basic Constraints</th><td id="cert-basic-constraints"></td></tr>
      </table>
    </div>

    <div class="cert-card">
      <div class="cert-card-header"><h2>Key Usage &amp; Extended Key Usage</h2></div>
      <div class="cert-usage-body">
        <div class="cert-usage-row">
          <span class="cert-usage-label">KEY USAGE</span>
          <div id="cert-key-usage" class="cert-tag-list"></div>
        </div>
        <div class="cert-usage-row">
          <span class="cert-usage-label">EXTENDED KEY USAGE</span>
          <div id="cert-eku" class="cert-tag-list"></div>
        </div>
      </div>
    </div>

    <div class="cert-card">
      <div class="cert-card-header"><h2>Fingerprints</h2></div>
      <table class="cert-table">
        <tr><th>SHA-256</th><td id="cert-sha256" class="cert-mono-wrap"></td></tr>
        <tr><th>SHA-1</th><td id="cert-sha1" class="cert-mono-wrap"></td></tr>
      </table>
    </div>

  </div>
</div>

<style>
.tool-page-container { max-width: 900px; }
.page-hero { margin-bottom: 0.25rem; }
.page-hero h1 { font-size: 2rem; font-weight: 800; letter-spacing: -0.02em; margin-bottom: 0.35rem; color: var(--fg, #e2e8f0); }
.page-hero p  { color: var(--muted, #94a3b8); font-size: 0.95rem; }

.cert-app { display: flex; flex-direction: column; gap: 1.5rem; }

.cert-card { background: var(--card, #1a2233); border: 1px solid var(--border, #2d3a4f); border-radius: 10px; overflow: hidden; }

.cert-card-header { padding: 1rem 1.5rem; border-bottom: 1px solid var(--border, #2d3a4f); display: flex; align-items: center; justify-content: space-between; gap: 1rem; flex-wrap: wrap; }
.cert-card-header h2 { font-size: 1rem; font-weight: 700; margin: 0; color: var(--fg, #e2e8f0); }
.cert-header-actions { display: flex; gap: 0.5rem; }

.cert-btn-primary { background: var(--accent, #3b82f6); border: none; border-radius: 7px; color: #fff; cursor: pointer; font-size: 0.875rem; font-weight: 700; padding: 0.55rem 1.25rem; transition: background 0.15s; white-space: nowrap; }
.cert-btn-primary:hover:not(:disabled) { background: #2563eb; }
.cert-btn-primary:disabled { opacity: 0.45; cursor: not-allowed; }

.cert-btn-secondary { background: transparent; border: 1px solid var(--border, #2d3a4f); border-radius: 7px; color: var(--muted, #94a3b8); cursor: pointer; font-size: 0.8rem; font-weight: 600; padding: 0.4rem 0.9rem; transition: color 0.15s, border-color 0.15s; white-space: nowrap; }
.cert-btn-secondary:hover { color: var(--fg, #e2e8f0); border-color: var(--accent, #3b82f6); }

.cert-btn-ghost { background: transparent; border: none; color: var(--muted, #94a3b8); cursor: pointer; font-size: 0.8rem; padding: 0.4rem 0.5rem; transition: color 0.15s; }
.cert-btn-ghost:hover { color: var(--fg, #e2e8f0); }

.cert-input-body { padding: 1.25rem 1.5rem; }
.cert-textarea { background: #0d1420; border: 1px solid var(--border, #2d3a4f); border-radius: 8px; box-sizing: border-box; color: var(--fg, #e2e8f0); display: block; font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.8rem; line-height: 1.6; outline: none; padding: 0.75rem 1rem; resize: vertical; transition: border-color 0.15s; width: 100%; }
.cert-textarea:focus { border-color: var(--accent, #3b82f6); }
.cert-textarea.drag-over { border-color: var(--accent, #3b82f6); background: rgba(59,130,246,0.04); }

.cert-input-footer { align-items: center; display: flex; gap: 1rem; justify-content: space-between; margin-top: 0.75rem; }
.cert-hint { color: var(--muted, #94a3b8); font-size: 0.8rem; }

.cert-error { background: rgba(239,68,68,0.08); border: 1px solid rgba(239,68,68,0.25); border-radius: 10px; color: #fca5a5; font-size: 0.875rem; padding: 1rem 1.25rem; }

.cert-table { border-collapse: collapse; font-size: 0.85rem; width: 100%; }
.cert-table th { border-bottom: 1px solid var(--border, #2d3a4f); color: var(--muted, #94a3b8); font-size: 0.72rem; font-weight: 700; letter-spacing: 0.06em; padding: 0.65rem 1.5rem; text-align: left; vertical-align: top; white-space: nowrap; width: 150px; }
.cert-table td { border-bottom: 1px solid var(--border, #2d3a4f); color: var(--fg, #e2e8f0); font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.82rem; padding: 0.65rem 1.5rem 0.65rem 0; vertical-align: top; word-break: break-all; }
.cert-table tr:last-child th, .cert-table tr:last-child td { border-bottom: none; }
.cert-mono-wrap { word-break: break-all; }

.cert-badge { border-radius: 5px; font-size: 0.72rem; font-weight: 700; padding: 0.2rem 0.6rem; white-space: nowrap; }
.cert-badge-warning { background: rgba(245,158,11,0.12); border: 1px solid rgba(245,158,11,0.3); color: #fbbf24; }

.cert-days-badge { border-radius: 5px; font-size: 0.72rem; font-weight: 700; padding: 0.25rem 0.65rem; white-space: nowrap; }
.cert-days-badge.valid   { background: rgba(16,185,129,0.1); border: 1px solid rgba(16,185,129,0.3); color: #34d399; }
.cert-days-badge.soon    { background: rgba(245,158,11,0.1); border: 1px solid rgba(245,158,11,0.3); color: #fbbf24; }
.cert-days-badge.expired { background: rgba(239,68,68,0.1); border: 1px solid rgba(239,68,68,0.3); color: #f87171; }

.cert-san-body { padding: 1.1rem 1.5rem; }
.cert-san-list { display: flex; flex-wrap: wrap; gap: 0.5rem; }
.cert-san-wrap { display: inline-flex; align-items: center; }
.cert-san-type { background: rgba(99,102,241,0.15); border: 1px solid rgba(99,102,241,0.3); border-radius: 5px 0 0 5px; border-right: none; color: #a5b4fc; font-size: 0.65rem; font-weight: 700; letter-spacing: 0.05em; padding: 0.22rem 0.4rem; }
.cert-san-chip { background: rgba(99,102,241,0.08); border: 1px solid rgba(99,102,241,0.3); border-left: none; border-radius: 0 5px 5px 0; color: #c7d2fe; font-family: 'SF Mono','Fira Code',ui-monospace,monospace; font-size: 0.8rem; padding: 0.22rem 0.6rem; }
.cert-san-ip .cert-san-type { background: rgba(6,182,212,0.12); border-color: rgba(6,182,212,0.3); color: #67e8f9; }
.cert-san-ip .cert-san-chip { background: rgba(6,182,212,0.07); border-color: rgba(6,182,212,0.3); color: #a5f3fc; }

.cert-usage-body { display: flex; flex-direction: column; gap: 0.85rem; padding: 1rem 1.5rem; }
.cert-usage-row { align-items: flex-start; display: flex; gap: 1rem; }
.cert-usage-label { color: var(--muted, #94a3b8); font-size: 0.65rem; font-weight: 700; letter-spacing: 0.08em; min-width: 130px; padding-top: 0.25rem; white-space: nowrap; }
.cert-tag-list { display: flex; flex: 1; flex-wrap: wrap; gap: 0.4rem; }
.cert-tag { background: rgba(59,130,246,0.1); border: 1px solid rgba(59,130,246,0.25); border-radius: 5px; color: #93c5fd; font-size: 0.78rem; padding: 0.2rem 0.6rem; }

@media (max-width: 640px) {
  .page-hero h1 { font-size: 1.6rem; }
  .cert-table th { padding: 0.55rem 1rem; width: 100px; }
  .cert-table td { padding: 0.55rem 0.75rem 0.55rem 0; }
  .cert-usage-row { flex-direction: column; gap: 0.4rem; }
  .cert-usage-label { min-width: auto; }
  .cert-input-footer { flex-direction: column; align-items: flex-start; }
}
</style>

<script>
(function () {
  'use strict';

  // ── OID registry ─────────────────────────────────────────────────────────────
  var OID = {
    '2.5.4.3':'CN','2.5.4.4':'SN','2.5.4.6':'C','2.5.4.7':'L','2.5.4.8':'ST',
    '2.5.4.9':'street','2.5.4.10':'O','2.5.4.11':'OU','2.5.4.12':'title',
    '2.5.4.17':'postalCode','2.5.4.42':'givenName',
    '1.2.840.113549.1.9.1':'emailAddress',
    '1.2.840.113549.1.1.1':'rsaEncryption',
    '1.2.840.113549.1.1.5':'sha1WithRSAEncryption',
    '1.2.840.113549.1.1.11':'sha256WithRSAEncryption',
    '1.2.840.113549.1.1.12':'sha384WithRSAEncryption',
    '1.2.840.113549.1.1.13':'sha512WithRSAEncryption',
    '1.2.840.113549.1.1.14':'sha224WithRSAEncryption',
    '1.3.101.112':'Ed25519','1.3.101.113':'Ed448',
    '1.2.840.10045.2.1':'ecPublicKey',
    '1.2.840.10045.4.3.1':'ecdsa-with-SHA224',
    '1.2.840.10045.4.3.2':'ecdsa-with-SHA256',
    '1.2.840.10045.4.3.3':'ecdsa-with-SHA384',
    '1.2.840.10045.4.3.4':'ecdsa-with-SHA512',
    '1.2.840.10045.3.1.7':'P-256','1.3.132.0.34':'P-384','1.3.132.0.35':'P-521',
    '2.5.29.14':'subjectKeyIdentifier','2.5.29.15':'keyUsage',
    '2.5.29.17':'subjectAltName','2.5.29.19':'basicConstraints',
    '2.5.29.31':'cRLDistributionPoints','2.5.29.32':'certificatePolicies',
    '2.5.29.35':'authorityKeyIdentifier','2.5.29.37':'extKeyUsage',
    '1.3.6.1.5.5.7.3.1':'TLS Server Auth','1.3.6.1.5.5.7.3.2':'TLS Client Auth',
    '1.3.6.1.5.5.7.3.3':'Code Signing','1.3.6.1.5.5.7.3.4':'Email Protection',
    '1.3.6.1.5.5.7.3.8':'Time Stamping','1.3.6.1.5.5.7.3.9':'OCSP Signing',
  };

  var KEY_USAGE_BITS = [
    'Digital Signature','Non Repudiation','Key Encipherment','Data Encipherment',
    'Key Agreement','Key Cert Sign','CRL Sign','Encipher Only','Decipher Only'
  ];

  // ── ASN.1 / DER parser ───────────────────────────────────────────────────────

  function parseDER(buf, offset) {
    var view = new Uint8Array(buf);
    var tagByte = view[offset];
    var tagClass = (tagByte >> 6) & 0x3;
    var constructed = !!(tagByte & 0x20);
    var tag = tagByte & 0x1f;
    var pos = offset + 1;
    if (tag === 0x1f) {
      tag = 0;
      while (pos < view.length) {
        var b = view[pos++];
        tag = (tag << 7) | (b & 0x7f);
        if (!(b & 0x80)) break;
      }
    }
    var length = 0;
    var lenByte = view[pos++];
    if (lenByte < 0x80) {
      length = lenByte;
    } else {
      var n = lenByte & 0x7f;
      for (var i = 0; i < n; i++) length = (length * 256) + view[pos++];
    }
    return { tag: tag, tagClass: tagClass, constructed: constructed, length: length, valueStart: pos, valueEnd: pos + length, end: pos + length };
  }

  function readChildren(buf, start, end) {
    var children = [], pos = start;
    while (pos < end) {
      var node = parseDER(buf, pos);
      children.push(node);
      pos = node.end;
    }
    return children;
  }

  function decodeOID(buf, node) {
    var bytes = new Uint8Array(buf, node.valueStart, node.length);
    var parts = [], val = 0, first = true;
    for (var i = 0; i < bytes.length; i++) {
      var b = bytes[i];
      val = (val * 128) + (b & 0x7f);
      if (!(b & 0x80)) {
        if (first) { parts.push(Math.floor(val / 40), val % 40); first = false; }
        else parts.push(val);
        val = 0;
      }
    }
    return parts.join('.');
  }

  function decodeString(buf, node) {
    var bytes = new Uint8Array(buf, node.valueStart, node.length);
    if (node.tag === 0x1e) {
      var s = '';
      for (var i = 0; i < bytes.length; i += 2) s += String.fromCharCode((bytes[i] << 8) | bytes[i+1]);
      return s;
    }
    return new TextDecoder('utf-8', { fatal: false }).decode(bytes);
  }

  function decodeTime(buf, node) {
    var bytes = new Uint8Array(buf, node.valueStart, node.length);
    var str = new TextDecoder().decode(bytes);
    var year, month, day, hour, min, sec;
    if (node.tag === 23) {
      year = parseInt(str.slice(0, 2), 10);
      year += year < 50 ? 2000 : 1900;
      month = parseInt(str.slice(2, 4), 10);
      day   = parseInt(str.slice(4, 6), 10);
      hour  = parseInt(str.slice(6, 8), 10);
      min   = parseInt(str.slice(8, 10), 10);
      sec   = parseInt(str.slice(10, 12), 10);
    } else {
      year  = parseInt(str.slice(0, 4), 10);
      month = parseInt(str.slice(4, 6), 10);
      day   = parseInt(str.slice(6, 8), 10);
      hour  = parseInt(str.slice(8, 10), 10);
      min   = parseInt(str.slice(10, 12), 10);
      sec   = parseInt(str.slice(12, 14), 10);
    }
    return new Date(Date.UTC(year, month - 1, day, hour, min, sec));
  }

  function decodeSerial(buf, node) {
    var bytes = Array.from(new Uint8Array(buf, node.valueStart, node.length));
    if (bytes[0] === 0) bytes = bytes.slice(1);
    return bytes.map(function(b) { return b.toString(16).padStart(2, '0'); }).join(':').toUpperCase();
  }

  function decodeRDN(buf, node) {
    var rdns = {};
    var sets = readChildren(buf, node.valueStart, node.valueEnd);
    for (var i = 0; i < sets.length; i++) {
      var set = sets[i];
      var attrs = readChildren(buf, set.valueStart, set.valueEnd);
      for (var j = 0; j < attrs.length; j++) {
        var attr = attrs[j];
        var parts = readChildren(buf, attr.valueStart, attr.valueEnd);
        if (parts.length < 2) continue;
        var oid  = decodeOID(buf, parts[0]);
        var name = OID[oid] || oid;
        rdns[name] = decodeString(buf, parts[1]);
      }
    }
    return rdns;
  }

  function decodeSAN(buf, node) {
    try {
      var sanSeq = parseDER(buf, node.valueStart);
      if (sanSeq.tag !== 0x10) return [];
      var gnodes = readChildren(buf, sanSeq.valueStart, sanSeq.valueEnd);
      var names = [];
      for (var i = 0; i < gnodes.length; i++) {
        var gn = gnodes[i];
        var bytes = new Uint8Array(buf, gn.valueStart, gn.length);
        if (gn.tag === 2) {
          names.push({ type: 'DNS', value: new TextDecoder().decode(bytes) });
        } else if (gn.tag === 7) {
          if (bytes.length === 4) {
            names.push({ type: 'IP', value: Array.from(bytes).join('.') });
          } else if (bytes.length === 16) {
            var hex = Array.from(bytes).map(function(b) { return b.toString(16).padStart(2,'0'); });
            var groups = [];
            for (var g = 0; g < 8; g++) groups.push(hex[g*2] + hex[g*2+1]);
            names.push({ type: 'IP', value: groups.join(':') });
          }
        } else if (gn.tag === 1) {
          names.push({ type: 'Email', value: new TextDecoder().decode(bytes) });
        } else if (gn.tag === 6) {
          names.push({ type: 'URI', value: new TextDecoder().decode(bytes) });
        }
      }
      return names;
    } catch (e) { return []; }
  }

  function decodeKeyUsage(buf, node) {
    try {
      var bitStr = parseDER(buf, node.valueStart);
      if (bitStr.tag !== 3) return [];
      var bytes = new Uint8Array(buf, bitStr.valueStart, bitStr.length);
      var unusedBits = bytes[0];
      var totalBits = (bytes.length - 1) * 8 - unusedBits;
      var used = [];
      for (var bitIdx = 0; bitIdx < Math.min(totalBits, KEY_USAGE_BITS.length); bitIdx++) {
        var byteIdx = 1 + Math.floor(bitIdx / 8);
        var bitPos  = 7 - (bitIdx % 8);
        if ((bytes[byteIdx] >> bitPos) & 1) used.push(KEY_USAGE_BITS[bitIdx]);
      }
      return used;
    } catch (e) { return []; }
  }

  function decodeEKU(buf, node) {
    try {
      var seq = parseDER(buf, node.valueStart);
      if (seq.tag !== 0x10) return [];
      return readChildren(buf, seq.valueStart, seq.valueEnd).map(function(o) {
        var oid = decodeOID(buf, o);
        return OID[oid] || oid;
      });
    } catch (e) { return []; }
  }

  function decodeBasicConstraints(buf, node) {
    try {
      var seq = parseDER(buf, node.valueStart);
      if (seq.tag !== 0x10) return { isCA: false, pathLen: null };
      var children = readChildren(buf, seq.valueStart, seq.valueEnd);
      var isCA = false, pathLen = null;
      for (var i = 0; i < children.length; i++) {
        var c = children[i];
        if (c.tag === 1) {
          isCA = new Uint8Array(buf, c.valueStart, 1)[0] !== 0;
        } else if (c.tag === 2) {
          pathLen = new Uint8Array(buf, c.valueStart, 1)[0];
        }
      }
      return { isCA: isCA, pathLen: pathLen };
    } catch (e) { return { isCA: false, pathLen: null }; }
  }

  function getRSAKeySize(buf, spkiNode) {
    try {
      var children = readChildren(buf, spkiNode.valueStart, spkiNode.valueEnd);
      if (children.length < 2) return null;
      var bitStr = children[1];
      var rsaSeq = parseDER(buf, bitStr.valueStart + 1);
      if (rsaSeq.tag !== 0x10) return null;
      var rsaChildren = readChildren(buf, rsaSeq.valueStart, rsaSeq.valueEnd);
      if (!rsaChildren.length) return null;
      var mod = rsaChildren[0];
      var len = mod.length;
      if (new Uint8Array(buf, mod.valueStart, 1)[0] === 0) len--;
      return len * 8;
    } catch (e) { return null; }
  }

  function pemToDER(pem) {
    var b64 = pem.replace(/-----[^-]+-----/g, '').replace(/\s+/g, '');
    var bin = atob(b64);
    var buf = new ArrayBuffer(bin.length);
    var view = new Uint8Array(buf);
    for (var i = 0; i < bin.length; i++) view[i] = bin.charCodeAt(i);
    return buf;
  }

  function toFingerprint(buf) {
    return Array.from(new Uint8Array(buf))
      .map(function(b) { return b.toString(16).padStart(2, '0'); })
      .join(':').toUpperCase();
  }

  async function decodeCert(pem) {
    var derBuf = pemToDER(pem);

    var certSeq = parseDER(derBuf, 0);
    var certChildren = readChildren(derBuf, certSeq.valueStart, certSeq.valueEnd);
    var tbs = certChildren[0];
    var tbsChildren = readChildren(derBuf, tbs.valueStart, tbs.valueEnd);

    var idx = 0;
    var version = 1;
    if (tbsChildren[idx].tagClass === 2 && tbsChildren[idx].tag === 0) {
      var verNode = parseDER(derBuf, tbsChildren[idx].valueStart);
      version = new Uint8Array(derBuf, verNode.valueStart, 1)[0] + 1;
      idx++;
    }

    var serialNode = tbsChildren[idx++];
    var sigAlgNode = tbsChildren[idx++];
    var issuerNode = tbsChildren[idx++];
    var validNode  = tbsChildren[idx++];
    var subjNode   = tbsChildren[idx++];
    var spkiNode   = tbsChildren[idx++];

    while (idx < tbsChildren.length && tbsChildren[idx].tagClass === 2 && tbsChildren[idx].tag < 3) idx++;

    var extensions = [];
    if (idx < tbsChildren.length && tbsChildren[idx].tagClass === 2 && tbsChildren[idx].tag === 3) {
      var extWrapper = tbsChildren[idx];
      var extSeq = parseDER(derBuf, extWrapper.valueStart);
      var extNodes = readChildren(derBuf, extSeq.valueStart, extSeq.valueEnd);
      for (var ei = 0; ei < extNodes.length; ei++) {
        var extParts = readChildren(derBuf, extNodes[ei].valueStart, extNodes[ei].valueEnd);
        var extOid = decodeOID(derBuf, extParts[0]);
        var valueNode = extParts[extParts.length - 1];
        extensions.push({ oid: extOid, name: OID[extOid] || extOid, valueNode: valueNode });
      }
    }

    var sigAlgOidNode = readChildren(derBuf, certChildren[1].valueStart, certChildren[1].valueEnd)[0];
    var sigAlgOid  = decodeOID(derBuf, sigAlgOidNode);
    var sigAlgName = OID[sigAlgOid] || sigAlgOid;

    var issuer  = decodeRDN(derBuf, issuerNode);
    var subject = decodeRDN(derBuf, subjNode);

    var validChildren = readChildren(derBuf, validNode.valueStart, validNode.valueEnd);
    var notBefore = decodeTime(derBuf, validChildren[0]);
    var notAfter  = decodeTime(derBuf, validChildren[1]);

    var spkiChildren = readChildren(derBuf, spkiNode.valueStart, spkiNode.valueEnd);
    var pkAlgChildren = readChildren(derBuf, spkiChildren[0].valueStart, spkiChildren[0].valueEnd);
    var pkAlgOid = decodeOID(derBuf, pkAlgChildren[0]);

    var keyInfo;
    if (pkAlgOid === '1.2.840.113549.1.1.1') {
      var rsaSize = getRSAKeySize(derBuf, spkiNode);
      keyInfo = 'RSA' + (rsaSize ? ' ' + rsaSize + '-bit' : '');
    } else if (pkAlgOid === '1.2.840.10045.2.1') {
      var curve = pkAlgChildren.length > 1 ? (OID[decodeOID(derBuf, pkAlgChildren[1])] || decodeOID(derBuf, pkAlgChildren[1])) : '';
      keyInfo = 'EC' + (curve ? ' (' + curve + ')' : '');
    } else if (pkAlgOid === '1.3.101.112') {
      keyInfo = 'Ed25519';
    } else if (pkAlgOid === '1.3.101.113') {
      keyInfo = 'Ed448';
    } else {
      keyInfo = OID[pkAlgOid] || pkAlgOid;
    }

    var sans = [], keyUsage = [], eku = [], basicConstraints = null;
    for (var xi = 0; xi < extensions.length; xi++) {
      var ext = extensions[xi];
      if (ext.name === 'subjectAltName')    sans = decodeSAN(derBuf, ext.valueNode);
      else if (ext.name === 'keyUsage')     keyUsage = decodeKeyUsage(derBuf, ext.valueNode);
      else if (ext.name === 'extKeyUsage')  eku = decodeEKU(derBuf, ext.valueNode);
      else if (ext.name === 'basicConstraints') basicConstraints = decodeBasicConstraints(derBuf, ext.valueNode);
    }

    var sha256Buf = await crypto.subtle.digest('SHA-256', derBuf);
    var sha1Buf   = await crypto.subtle.digest('SHA-1',   derBuf);

    return {
      version: version,
      serial: decodeSerial(derBuf, serialNode),
      sigAlgName: sigAlgName,
      issuer: issuer, subject: subject,
      notBefore: notBefore, notAfter: notAfter,
      keyInfo: keyInfo,
      sans: sans, keyUsage: keyUsage, eku: eku,
      basicConstraints: basicConstraints,
      sha256: toFingerprint(sha256Buf),
      sha1:   toFingerprint(sha1Buf),
    };
  }

  // ── UI helpers ───────────────────────────────────────────────────────────────

  function set(id, val) { var el = document.getElementById(id); if (el) el.textContent = val || '—'; }
  function show(el) { el.style.display = ''; }
  function hide(el) { el.style.display = 'none'; }

  function formatDate(d) {
    if (!d) return '—';
    return d.toUTCString().replace(' GMT', ' UTC');
  }

  function renderSANs(sans) {
    var container = document.getElementById('cert-sans');
    container.innerHTML = '';
    if (!sans || !sans.length) { container.textContent = '—'; return; }
    sans.forEach(function(san) {
      var typeEl = document.createElement('span');
      typeEl.className = 'cert-san-type';
      typeEl.textContent = san.type;
      var chipEl = document.createElement('span');
      chipEl.className = 'cert-san-chip';
      chipEl.textContent = san.value;
      var wrap = document.createElement('span');
      wrap.className = 'cert-san-wrap' + (san.type === 'IP' ? ' cert-san-ip' : '');
      wrap.appendChild(typeEl);
      wrap.appendChild(chipEl);
      container.appendChild(wrap);
    });
  }

  function renderList(id, items) {
    var el = document.getElementById(id);
    el.innerHTML = '';
    if (!items || !items.length) { el.textContent = '—'; return; }
    items.forEach(function(item) {
      var span = document.createElement('span');
      span.className = 'cert-tag';
      span.textContent = item;
      el.appendChild(span);
    });
  }

  async function decode() {
    var pem = document.getElementById('cert-input').value.trim();
    if (!pem) return;
    var decodeBtn = document.getElementById('cert-decode-btn');
    var errorEl   = document.getElementById('cert-error');
    var resultEl  = document.getElementById('cert-result');
    decodeBtn.disabled = true;
    hide(errorEl);
    hide(resultEl);
    try {
      var cert = await decodeCert(pem);
      var now  = Date.now();
      var days = Math.floor((cert.notAfter.getTime() - now) / 86400000);
      var isExpired = days < 0;
      var isSoon    = days >= 0 && days < 30;

      set('cert-subject-cn', cert.subject.CN);
      set('cert-subject-o',  cert.subject.O);
      set('cert-subject-ou', cert.subject.OU);
      set('cert-subject-l',  cert.subject.L);
      set('cert-subject-st', cert.subject.ST);
      set('cert-subject-c',  cert.subject.C);

      set('cert-issuer-cn', cert.issuer.CN);
      set('cert-issuer-o',  cert.issuer.O);
      set('cert-issuer-c',  cert.issuer.C);

      var ssEl = document.getElementById('cert-self-signed');
      if (ssEl) ssEl.style.display = (cert.subject.CN && cert.subject.CN === cert.issuer.CN && cert.subject.O === cert.issuer.O) ? '' : 'none';

      set('cert-not-before', formatDate(cert.notBefore));
      set('cert-not-after',  formatDate(cert.notAfter));

      var daysEl = document.getElementById('cert-days-badge');
      if (daysEl) {
        daysEl.textContent = isExpired ? 'Expired ' + Math.abs(days) + 'd ago' : days + 'd remaining';
        daysEl.className = 'cert-days-badge ' + (isExpired ? 'expired' : isSoon ? 'soon' : 'valid');
      }

      set('cert-version', 'v' + cert.version);
      set('cert-serial', cert.serial);
      set('cert-sig-alg', cert.sigAlgName);
      set('cert-key-info', cert.keyInfo);

      var bcEl = document.getElementById('cert-basic-constraints');
      if (bcEl) {
        if (cert.basicConstraints) {
          var bc = cert.basicConstraints;
          bcEl.textContent = bc.isCA
            ? 'CA: true' + (bc.pathLen !== null ? ', pathLen: ' + bc.pathLen : '')
            : 'CA: false (end-entity)';
        } else {
          bcEl.textContent = '—';
        }
      }

      renderSANs(cert.sans);
      renderList('cert-key-usage', cert.keyUsage);
      renderList('cert-eku', cert.eku);

      set('cert-sha256', cert.sha256);
      set('cert-sha1',   cert.sha1);

      show(resultEl);
    } catch (e) {
      errorEl.textContent = '⚠ ' + (e.message || 'Failed to decode certificate — make sure the PEM is complete and valid.');
      show(errorEl);
    } finally {
      document.getElementById('cert-decode-btn').disabled = false;
    }
  }

  // ── Example cert: ISRG Root X1 (Let's Encrypt root CA) ───────────────────────
  var EXAMPLE_PEM = [
    '-----BEGIN CERTIFICATE-----',
    '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',
    '-----END CERTIFICATE-----'
  ].join('\n');

  // ── Wire up events ────────────────────────────────────────────────────────────

  document.getElementById('cert-decode-btn').addEventListener('click', decode);
  document.getElementById('cert-example-btn').addEventListener('click', function() {
    document.getElementById('cert-input').value = EXAMPLE_PEM;
    decode();
  });
  document.getElementById('cert-clear-btn').addEventListener('click', function() {
    document.getElementById('cert-input').value = '';
    hide(document.getElementById('cert-error'));
    hide(document.getElementById('cert-result'));
  });

  document.getElementById('cert-input').addEventListener('paste', function() {
    setTimeout(decode, 60);
  });

  // Drag-and-drop .pem / .crt files
  var ta = document.getElementById('cert-input');
  ta.addEventListener('dragover', function(e) { e.preventDefault(); ta.classList.add('drag-over'); });
  ta.addEventListener('dragleave', function() { ta.classList.remove('drag-over'); });
  ta.addEventListener('drop', function(e) {
    e.preventDefault();
    ta.classList.remove('drag-over');
    var file = e.dataTransfer.files[0];
    if (!file) return;
    var reader = new FileReader();
    reader.onload = function(ev) {
      ta.value = ev.target.result;
      decode();
    };
    reader.readAsText(file);
  });
})();
</script>
