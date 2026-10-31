---
title: "IP Geolocation"
description: "Look up any IP address or domain — country, city, ISP, ASN, timezone, and map location."
status: "live"
icon: "🌍"
tags: ["ip", "geolocation", "networking", "asn", "isp"]
weight: 5
---

<div class="geo-app">

  <div class="page-hero">
    <h1>IP Geolocation</h1>
    <p>Look up any IP address or domain — country, city, ISP, ASN, timezone, and map location.</p>
  </div>

  <!-- Input -->
  <div class="geo-card" id="geo-input-card">
    <div class="geo-card-header">
      <h2>IP / Domain Lookup</h2>
      <p>Enter an IPv4, IPv6 address, or domain name.</p>
    </div>
    <div class="geo-form">
      <div class="geo-input-row">
        <input
          id="geo-input"
          class="geo-input-field"
          type="text"
          placeholder="8.8.8.8 or example.com"
          spellcheck="false"
          autocorrect="off"
          autocapitalize="off"
        />
        <button id="geo-btn" class="geo-btn-primary">Look up →</button>
        <button id="myip-btn" class="geo-btn-secondary">My IP</button>
      </div>
      <div class="geo-examples-row">
        <span class="geo-label">EXAMPLES:</span>
        <button class="geo-chip" data-val="8.8.8.8">8.8.8.8</button>
        <button class="geo-chip" data-val="1.1.1.1">1.1.1.1</button>
        <button class="geo-chip" data-val="github.com">github.com</button>
        <button class="geo-chip" data-val="2606:4700:4700::1111">2606:4700::1111</button>
      </div>
    </div>
  </div>

  <!-- Loading -->
  <div id="geo-loading" class="geo-loading" style="display:none">
    <span class="geo-spinner"></span>
    Resolving…
  </div>

  <!-- Error -->
  <div id="geo-error" class="geo-error" style="display:none"></div>

  <div id="geo-result" style="display:none"><div class="geo-card geo-summary-card"><div class="geo-summary-layout"><span id="res-flag" class="geo-flag"></span><div class="geo-summary-main"><div id="res-ip" class="geo-ip-mono"></div><div id="res-loc" class="geo-loc-line"></div></div><div id="res-type-badge" class="geo-type-badge"></div></div></div><div class="geo-two-col"><div class="geo-card"><div class="geo-card-header"><h2>Location</h2></div><table class="geo-table"><tbody><tr><th>Country</th><td id="res-country"></td></tr><tr><th>Region</th><td id="res-region"></td></tr><tr><th>City</th><td id="res-city"></td></tr><tr><th>Postal</th><td id="res-postal"></td></tr><tr><th>Timezone</th><td id="res-tz"></td></tr><tr><th>Coordinates</th><td id="res-coords"></td></tr></tbody></table></div><div class="geo-card"><div class="geo-card-header"><h2>Network</h2></div><table class="geo-table"><tbody><tr><th>ISP</th><td id="res-isp"></td></tr><tr><th>Organization</th><td id="res-org"></td></tr><tr><th>ASN</th><td id="res-asn"></td></tr><tr><th>Domain</th><td id="res-domain"></td></tr><tr><th>Type</th><td id="res-conn-type"></td></tr></tbody></table></div></div><div class="geo-card geo-map-card"><div class="geo-card-header"><h2>Map</h2><a id="res-map-link" href="#" target="_blank" rel="noopener" class="geo-map-ext-link">Open in OpenStreetMap →</a></div><iframe id="res-map" class="geo-map-iframe" loading="lazy" referrerpolicy="no-referrer" src="about:blank"></iframe></div></div>

</div>

<style>
.tool-page-container { max-width: 900px; }

.tool-breadcrumb {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--muted, #94a3b8);
  margin-bottom: 2rem;
}
.tool-breadcrumb a { color: var(--muted, #94a3b8); }
.tool-breadcrumb a:hover { color: var(--fg, #e2e8f0); }
.tool-breadcrumb span:not([class]) { color: var(--border, #2d3a4f); }

.geo-app {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.page-hero { margin-bottom: 0.25rem; }
.page-hero h1 { font-size: 2rem; font-weight: 800; letter-spacing: -0.02em; margin-bottom: 0.35rem; color: var(--fg, #e2e8f0); }
.page-hero p  { color: var(--muted, #94a3b8); font-size: 0.95rem; }

.geo-card {
  background: var(--card, #1a2233);
  border: 1px solid var(--border, #2d3a4f);
  border-radius: 10px;
  overflow: hidden;
}

.geo-card-header {
  padding: 1.1rem 1.5rem 1rem;
  border-bottom: 1px solid var(--border, #2d3a4f);
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.geo-card-header h2 { font-size: 1rem; font-weight: 700; margin: 0; color: var(--fg, #e2e8f0); }
.geo-card-header p  { color: var(--muted, #94a3b8); font-size: 0.825rem; margin: 0.2rem 0 0; }

.geo-form { padding: 1.25rem 1.5rem; }

.geo-input-row { display: flex; gap: 0.6rem; margin-bottom: 1rem; flex-wrap: wrap; }

.geo-input-field {
  flex: 1;
  min-width: 220px;
  background: #0d1420;
  border: 1px solid var(--border, #2d3a4f);
  border-radius: 8px;
  color: var(--fg, #e2e8f0);
  font-family: 'SF Mono', 'Fira Code', ui-monospace, monospace;
  font-size: 0.95rem;
  padding: 0.6rem 1rem;
  outline: none;
  transition: border-color 0.15s;
}
.geo-input-field:focus { border-color: var(--accent, #3b82f6); }

.geo-btn-primary {
  background: var(--accent, #3b82f6);
  border: none;
  border-radius: 8px;
  color: #fff;
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 700;
  padding: 0.6rem 1.3rem;
  white-space: nowrap;
  transition: background 0.15s, opacity 0.15s;
}
.geo-btn-primary:hover:not(:disabled) { background: #2563eb; }
.geo-btn-primary:disabled { opacity: 0.45; cursor: not-allowed; }

.geo-btn-secondary {
  background: transparent;
  border: 1px solid var(--border, #2d3a4f);
  border-radius: 8px;
  color: var(--muted, #94a3b8);
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 600;
  padding: 0.6rem 1rem;
  white-space: nowrap;
  transition: color 0.15s, border-color 0.15s;
}
.geo-btn-secondary:hover { color: var(--fg, #e2e8f0); border-color: var(--accent, #3b82f6); }

.geo-examples-row { display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; }
.geo-label { font-size: 0.68rem; font-weight: 700; letter-spacing: 0.07em; color: var(--muted, #94a3b8); }
.geo-chip {
  background: transparent;
  border: 1px solid var(--border, #2d3a4f);
  border-radius: 6px;
  color: var(--muted, #94a3b8);
  cursor: pointer;
  font-family: 'SF Mono', 'Fira Code', ui-monospace, monospace;
  font-size: 0.75rem;
  padding: 0.2rem 0.55rem;
  transition: color 0.15s, border-color 0.15s;
}
.geo-chip:hover { color: var(--fg, #e2e8f0); border-color: var(--accent, #3b82f6); }

.geo-loading {
  align-items: center;
  background: var(--card, #1a2233);
  border: 1px solid var(--border, #2d3a4f);
  border-radius: 10px;
  color: var(--muted, #94a3b8);
  display: flex;
  font-size: 0.875rem;
  gap: 0.75rem;
  padding: 1.25rem 1.5rem;
}
.geo-spinner {
  animation: geo-spin 0.7s linear infinite;
  border: 2px solid var(--border, #2d3a4f);
  border-top-color: var(--accent, #3b82f6);
  border-radius: 50%;
  flex-shrink: 0;
  height: 16px;
  width: 16px;
}
@keyframes geo-spin { to { transform: rotate(360deg); } }

.geo-error {
  background: rgba(239,68,68,0.08);
  border: 1px solid rgba(239,68,68,0.25);
  border-radius: 10px;
  color: #fca5a5;
  font-size: 0.875rem;
  padding: 1rem 1.25rem;
}

/* Summary */
.geo-summary-card { padding: 1.25rem 1.5rem; }
.geo-summary-layout { display: flex; align-items: center; gap: 1.25rem; }
.geo-flag { font-size: 2.5rem; flex-shrink: 0; line-height: 1; }
.geo-summary-main { flex: 1; min-width: 0; }
.geo-ip-mono {
  color: var(--fg, #e2e8f0);
  font-family: 'SF Mono', 'Fira Code', ui-monospace, monospace;
  font-size: 1.1rem;
  font-weight: 700;
  margin-bottom: 0.2rem;
}
.geo-loc-line { color: var(--muted, #94a3b8); font-size: 0.9rem; }
.geo-type-badge {
  background: rgba(6,182,212,0.1);
  border: 1px solid rgba(6,182,212,0.28);
  border-radius: 4px;
  color: var(--cyan, #06b6d4);
  font-size: 0.72rem;
  font-weight: 700;
  padding: 0.2rem 0.55rem;
  white-space: nowrap;
}

/* Two-col grid */
.geo-two-col {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1.5rem;
}

/* Detail table */
.geo-table { border-collapse: collapse; font-size: 0.85rem; width: 100%; }
.geo-table th {
  color: var(--muted, #94a3b8);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  padding: 0.6rem 1.5rem;
  text-align: left;
  vertical-align: top;
  white-space: nowrap;
  width: 110px;
  border-bottom: 1px solid var(--border, #2d3a4f);
}
.geo-table td {
  color: var(--fg, #e2e8f0);
  font-family: 'SF Mono', 'Fira Code', ui-monospace, monospace;
  font-size: 0.82rem;
  padding: 0.6rem 1.5rem 0.6rem 0;
  vertical-align: top;
  border-bottom: 1px solid var(--border, #2d3a4f);
  word-break: break-all;
}
.geo-table tr:last-child th,
.geo-table tr:last-child td { border-bottom: none; }

/* Map */
.geo-map-ext-link { font-size: 0.8rem; color: var(--muted, #94a3b8); }
.geo-map-ext-link:hover { color: var(--cyan, #06b6d4); }
.geo-map-iframe {
  border: none;
  display: block;
  height: 360px;
  width: 100%;
  filter: invert(0.9) hue-rotate(180deg) brightness(0.85) saturate(0.8);
}

@media (max-width: 640px) {
  .geo-two-col { grid-template-columns: 1fr; }
  .geo-map-iframe { height: 260px; }
  .page-hero h1 { font-size: 1.6rem; }
}
</style>

<script>
(function () {
  const inputEl   = document.getElementById('geo-input');
  const btnLookup = document.getElementById('geo-btn');
  const btnMyIp   = document.getElementById('myip-btn');
  const loading   = document.getElementById('geo-loading');
  const errorEl   = document.getElementById('geo-error');
  const resultEl  = document.getElementById('geo-result');

  function show(el)  { el.style.display = ''; }
  function hide(el)  { el.style.display = 'none'; }
  function setText(id, val) { const el = document.getElementById(id); if (el) el.textContent = val || '—'; }

  function setLoading(on) {
    btnLookup.disabled = on;
    btnMyIp.disabled   = on;
    on ? show(loading) : hide(loading);
  }

  function regionFlag(cc) {
    return [...cc.toUpperCase()].map(c => String.fromCodePoint(0x1F1E0 - 65 + c.charCodeAt(0))).join('');
  }

  // Resolve domain names to IPs via Cloudflare DNS-over-HTTPS
  async function resolveDomain(host) {
    const r = await fetch(
      `https://cloudflare-dns.com/dns-query?name=${encodeURIComponent(host)}&type=A`,
      { headers: { Accept: 'application/dns-json' }, signal: AbortSignal.timeout(5000) }
    );
    const d = await r.json();
    const ip = d.Answer?.find(a => a.type === 1)?.data;
    if (!ip) throw new Error(`Could not resolve ${host}`);
    return ip;
  }

  async function tryIpwho(ip) {
    const url = ip ? `https://ipwho.is/${encodeURIComponent(ip)}` : 'https://ipwho.is/';
    const res  = await fetch(url, { signal: AbortSignal.timeout(8000) });
    const data = await res.json();
    if (!data.success) throw new Error(data.message || 'Lookup failed');
    return data;
  }

  async function tryIpapi(ip) {
    const url = ip ? `https://ipapi.co/${encodeURIComponent(ip)}/json/` : 'https://ipapi.co/json/';
    const res  = await fetch(url, { signal: AbortSignal.timeout(8000) });
    const data = await res.json();
    if (data.error) throw new Error(data.reason || 'Lookup failed');
    // normalise to ipwho shape
    return {
      ip:           data.ip,
      type:         data.version,
      country:      data.country_name,
      country_code: data.country_code,
      region:       data.region,
      city:         data.city,
      postal:       data.postal,
      latitude:     data.latitude,
      longitude:    data.longitude,
      flag:         { emoji: data.country_code ? regionFlag(data.country_code) : '' },
      timezone:     { id: data.timezone },
      connection:   { isp: data.org, org: data.org, asn: data.asn?.replace('AS',''), domain: null },
    };
  }

  async function lookup(query) {
    setLoading(true);
    hide(errorEl);
    hide(resultEl);
    try {
      // Resolve domain names to IP first (geo APIs only accept IPs)
      let target = query;
      if (query && !/^[\d.:a-fA-F]+$/.test(query)) {
        target = await resolveDomain(query);
      }
      let data;
      try       { data = await tryIpwho(target); }
      catch (e) { data = await tryIpapi(target); }
      render(data);
      show(resultEl);
    } catch (e) {
      errorEl.textContent = '⚠ ' + (e.message || 'Request failed');
      show(errorEl);
    } finally {
      setLoading(false);
    }
  }

  function render(d) {
    const flag = d.flag?.emoji || '';
    const type = d.type || (d.ip?.includes(':') ? 'IPv6' : 'IPv4');
    const loc  = [d.city, d.region, d.country].filter(Boolean).join(', ');

    document.getElementById('res-flag').textContent       = flag;
    document.getElementById('res-ip').textContent         = d.ip || '—';
    document.getElementById('res-loc').textContent        = loc || '—';
    document.getElementById('res-type-badge').textContent = type;

    setText('res-country',   d.country   ? `${flag} ${d.country} (${d.country_code})` : null);
    setText('res-region',    d.region);
    setText('res-city',      d.city);
    setText('res-postal',    d.postal);
    setText('res-tz',        d.timezone?.id);
    setText('res-isp',       d.connection?.isp);
    setText('res-org',       d.connection?.org);
    setText('res-asn',       d.connection?.asn ? (String(d.connection.asn).startsWith('AS') ? d.connection.asn : `AS${d.connection.asn}`) : null);
    setText('res-domain',    d.connection?.domain);
    setText('res-conn-type', d.type);

    const lat = d.latitude;
    const lon = d.longitude;

    if (lat != null && lon != null) {
      document.getElementById('res-coords').textContent = `${lat.toFixed(4)}, ${lon.toFixed(4)}`;

      const zoom  = 10;
      const delta = 1.5;
      const bbox  = `${lon - delta},${lat - delta},${lon + delta},${lat + delta}`;
      const mapSrc = `https://www.openstreetmap.org/export/embed.html?bbox=${bbox}&layer=mapnik&marker=${lat},${lon}`;
      document.getElementById('res-map').src = mapSrc;

      const extLink = document.getElementById('res-map-link');
      extLink.href  = `https://www.openstreetmap.org/?mlat=${lat}&mlon=${lon}#map=${zoom}/${lat}/${lon}`;
    } else {
      setText('res-coords', null);
      document.getElementById('res-map').src = 'about:blank';
    }
  }

  function submit() {
    const q = inputEl.value.trim();
    lookup(q || '');
  }

  btnLookup.addEventListener('click', submit);
  btnMyIp.addEventListener('click', () => { inputEl.value = ''; lookup(''); });
  inputEl.addEventListener('keydown', e => { if (e.key === 'Enter') submit(); });

  document.querySelectorAll('.geo-chip').forEach(chip => {
    chip.addEventListener('click', () => {
      inputEl.value = chip.dataset.val;
      lookup(chip.dataset.val);
    });
  });
})();
</script>
