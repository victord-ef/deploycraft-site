---
title: "Deploying Suricata as a Network IDS/IPS — Part 2"
date: 2026-09-01
description: "Install Suricata on Linux, load and manage the Emerging Threats ruleset, configure EVE JSON alerting, tune rules to eliminate false positives, and forward alerts to a log aggregator."
cluster: "Network Security"
series: "IDS/IPS"
part: 2
difficulty: "intermediate"
duration: "50 min"
tags: ["ids", "ips", "suricata", "network-security", "intrusion-detection", "emerging-threats", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/network-security/ids-ips-architecture-detection-models-part-1/) you learned IDS/IPS architecture, detection models, and sensor placement. In Part 2 you will deploy Suricata as a working network IDS/IPS: install it, configure it to inspect real traffic, load the Emerging Threats Open ruleset, produce structured EVE JSON alerts, tune rules to suppress false positives, and forward alerts to a log aggregator.

## Prerequisites

- Completed [Part 1](/tutorials/network-security/ids-ips-architecture-detection-models-part-1/)
- A Linux host (Ubuntu 22.04 or Debian 12) with at least 2 CPU cores and 4GB RAM
- A monitored network interface (physical NIC or a VM with a second interface)
- `sudo` access

---

## Step 1 — Install Suricata

Install from the official OISF PPA for the latest stable release:

```bash
sudo apt-get install -y software-properties-common
sudo add-apt-repository ppa:oisf/suricata-stable
sudo apt-get update
sudo apt-get install -y suricata suricata-update
```

Verify the installation:

```bash
suricata --build-info | head -20
# Suricata 7.x.x
# PCRE version: ...
# AF_PACKET support: yes
# NFQ support: yes  (needed for IPS inline mode)
```

Check the default configuration:

```bash
suricata --list-runmodes
# Available run modes:
#   ids-pcap    : Read packets from pcap file
#   ids-af-packet : AF_PACKET IDS mode
#   ips-nfqueue : NFQ IPS inline mode
```

---

## Step 2 — Configure the network interface

Suricata uses AF_PACKET for zero-copy packet capture. Configure it in `/etc/suricata/suricata.yaml`:

```yaml
# /etc/suricata/suricata.yaml — key sections

af-packet:
  - interface: eth0          # replace with your monitored interface
    cluster-id: 99
    cluster-type: cluster_flow    # hash flows to the same thread
    defrag: yes
    use-mmap: yes
    tpacket-v3: yes
    ring-size: 200000
    block-size: 32768
    threads: auto             # one thread per CPU core

# For IPS inline mode (bridges two interfaces):
# af-packet:
#   - interface: eth0
#     copy-mode: ips
#     copy-iface: eth1
#   - interface: eth1
#     copy-mode: ips
#     copy-iface: eth0

threading:
  set-cpu-affinity: yes
  cpu-affinity:
    - management-cpu-set:
        cpu: [0]
    - receive-cpu-set:
        cpu: [1]
    - worker-cpu-set:
        cpu: [2, 3]

# Disable stream reassembly bypass for encrypted traffic inspection
stream:
  bypass: no
  memcap: 1gb
  checksum-validation: no
```

Set your internal network ranges — rules use `$HOME_NET` to distinguish internal from external:

```yaml
vars:
  address-groups:
    HOME_NET: "[10.0.0.0/8,172.16.0.0/12,192.168.0.0/16]"
    EXTERNAL_NET: "!$HOME_NET"
    HTTP_SERVERS: "$HOME_NET"
    SMTP_SERVERS: "$HOME_NET"
    SQL_SERVERS: "$HOME_NET"
    DNS_SERVERS: "$HOME_NET"
```

---

## Step 3 — Load the Emerging Threats ruleset

`suricata-update` manages rulesets. The Emerging Threats Open ruleset is free and covers the most common threats:

```bash
# Download and install rules
sudo suricata-update

# List available rule sources
sudo suricata-update list-sources

# Enable additional free sources
sudo suricata-update enable-source et/open
sudo suricata-update enable-source oisf/trafficid

# Update and merge all enabled sources
sudo suricata-update update

# Check how many rules are loaded
sudo suricata-update list-enabled-sources
ls -la /var/lib/suricata/rules/suricata.rules
wc -l /var/lib/suricata/rules/suricata.rules
# ~50,000+ rules
```

Point Suricata at the rule file in `suricata.yaml`:

```yaml
rule-files:
  - /var/lib/suricata/rules/suricata.rules
```

Test the configuration before starting:

```bash
sudo suricata -T -c /etc/suricata/suricata.yaml -v
# Configuration provided was successfully loaded. Exiting.
```

---

## Step 4 — Configure EVE JSON output

EVE JSON is Suricata's structured alert format. Configure it for both alerts and network metadata:

```yaml
# /etc/suricata/suricata.yaml — outputs section

outputs:
  - eve-log:
      enabled: yes
      filetype: regular
      filename: /var/log/suricata/eve.json
      types:
        - alert:
            payload: yes          # base64 payload of matching packet
            payload-printable: yes
            packet: yes
            metadata: yes
            tagged-packets: yes
        - anomaly:
            enabled: yes
            types:
              - decode
              - stream
              - applayer
        - http:
            extended: yes
        - dns:
            query: yes
            answer: yes
        - tls:
            extended: yes         # JA3/JA3S fingerprints, SNI, cert details
        - files:
            force-magic: yes
        - ssh
        - smtp
        - flow

  # Fast log for quick grep during incidents
  - fast:
      enabled: yes
      filename: /var/log/suricata/fast.log
      append: yes
```

---

## Step 5 — Start Suricata and verify

```bash
# Start Suricata in IDS mode (AF_PACKET, promiscuous)
sudo systemctl enable suricata
sudo systemctl start suricata

# Watch the stats output to confirm traffic is flowing
sudo tail -f /var/log/suricata/stats.log

# Watch live alerts
sudo tail -f /var/log/suricata/fast.log

# Generate test traffic to confirm detection is working
# The EICAR test string triggers anti-malware rules
curl -s http://www.eicar.org/download/eicar.com -o /dev/null

# Check for an alert
grep "EICAR" /var/log/suricata/fast.log
# Expected: alert for EICAR test file download
```

Check the stats log for key counters:

```bash
grep -E "capture.kernel_packets|decoder.pkts|detect.alert" /var/log/suricata/stats.log | tail -20
# capture.kernel_packets | Total | ...
# decoder.pkts | Total | ...
# detect.alert | Total | ...
```

If `capture.kernel_drops` is non-zero and growing, the sensor is dropping packets — increase `ring-size` or add more threads.

---

## Step 6 — Write a custom rule

Custom rules go in `/etc/suricata/rules/local.rules`. Reference this file in `suricata.yaml`:

```yaml
rule-files:
  - /var/lib/suricata/rules/suricata.rules
  - /etc/suricata/rules/local.rules
```

Write a rule to detect a specific threat in your environment:

```bash
cat > /etc/suricata/rules/local.rules << 'EOF'
# Detect outbound connections to known C2 port from internal hosts
alert tcp $HOME_NET any -> $EXTERNAL_NET 4444 (
  msg:"Possible reverse shell — outbound TCP/4444";
  flow:to_server,established;
  threshold:type limit, track by_src, count 1, seconds 60;
  classtype:trojan-activity;
  sid:9000001;
  rev:1;
)

# Detect base64-encoded PowerShell download cradles in HTTP
alert http $EXTERNAL_NET any -> $HOME_NET any (
  msg:"PowerShell encoded download cradle in HTTP response";
  flow:to_client,established;
  file.data;
  content:"powershell";
  nocase;
  content:"-enc";
  nocase;
  within:100;
  classtype:trojan-activity;
  sid:9000002;
  rev:1;
)

# Detect DNS queries to long subdomains (DNS tunneling indicator)
alert dns any any -> any any (
  msg:"Long DNS query — possible DNS tunneling";
  dns.query;
  content:".";
  pcre:"/^[a-z0-9]{30,}\./i";
  classtype:policy-violation;
  sid:9000003;
  rev:1;
)
EOF
```

Reload rules without restarting Suricata:

```bash
sudo kill -USR2 $(pidof suricata)
# Suricata reloads rules live — no traffic gap
```

---

## Step 7 — Tune rules to suppress false positives

False positives produce alert fatigue and cause real alerts to be ignored. Suppress them using threshold and suppress configuration.

### Suppress by source IP

Suppress alerts from known scanners (vulnerability scanners, monitoring systems):

```bash
cat >> /etc/suricata/threshold.conf << 'EOF'
# Suppress vulnerability scanner alerts from internal scanner
suppress gen_id 1, sig_id 2002910, track by_src, ip 10.0.1.50

# Suppress SSH brute force alerts from the jump host (expected high-volume SSH)
suppress gen_id 1, sig_id 2001219, track by_src, ip 10.0.1.1/32
EOF
```

### Rate limiting with threshold

Reduce noise from high-volume rules by limiting how often an alert fires:

```bash
cat >> /etc/suricata/threshold.conf << 'EOF'
# Only alert once per minute per source for port scan rules
threshold gen_id 1, sig_id 2002992, type threshold, track by_src, count 1, seconds 60

# Alert at most 5 times total from a given src for low-priority rules
threshold gen_id 1, sig_id 2013028, type limit, track by_src, count 5, seconds 3600
EOF
```

### Disable noisy rules globally

Find the rules generating the most alerts and disable them if not relevant to your environment:

```bash
# Count alerts per signature over the last hour
jq -r '.alert.signature_id' /var/log/suricata/eve.json \
  | sort | uniq -c | sort -rn | head -20

# Disable a specific rule by SID in suricata-update
cat > /etc/suricata/disable.conf << 'EOF'
# Disable rules for protocols not in use
2012648  # ET POLICY RDP connection to external
2001219  # ET SCAN Potential SSH Scan
EOF

sudo suricata-update --disable-conf /etc/suricata/disable.conf
sudo kill -USR2 $(pidof suricata)
```

---

## Step 8 — Forward alerts to a log aggregator

Ship EVE JSON to Elasticsearch (ELK) or a SIEM using Filebeat:

```bash
apt-get install -y filebeat

cat > /etc/filebeat/filebeat.yml << 'EOF'
filebeat.inputs:
  - type: log
    enabled: true
    paths:
      - /var/log/suricata/eve.json
    json.keys_under_root: true
    json.add_error_key: true
    fields:
      logtype: suricata
    fields_under_root: true

output.elasticsearch:
  hosts: ["https://elasticsearch:9200"]
  username: "suricata_shipper"
  password: "${ES_PASSWORD}"
  index: "suricata-%{+yyyy.MM.dd}"

setup.template.name: "suricata"
setup.template.pattern: "suricata-*"
EOF

systemctl enable filebeat && systemctl start filebeat
```

Or forward to Loki with Promtail:

```yaml
# /etc/promtail/config.yml
scrape_configs:
  - job_name: suricata
    static_configs:
      - targets: [localhost]
        labels:
          job: suricata
          __path__: /var/log/suricata/eve.json
    pipeline_stages:
      - json:
          expressions:
            event_type: event_type
            src_ip: src_ip
            signature: alert.signature
            severity: alert.severity
      - labels:
          event_type:
          severity:
      - output:
          source: message
```

---

## Step 9 — IPS inline mode with NFQ

To switch from IDS (detect-only) to IPS (blocking), route traffic through Suricata using netfilter queue (NFQ):

```bash
# Redirect traffic to NFQ queue 0
iptables -I FORWARD -j NFQUEUE --queue-num 0
iptables -I INPUT -j NFQUEUE --queue-num 0
iptables -I OUTPUT -j NFQUEUE --queue-num 0

# Start Suricata in NFQ IPS mode
sudo suricata --runmode=ips-nfqueue -q 0 -c /etc/suricata/suricata.yaml
```

In `suricata.yaml`, rules must specify `drop` instead of `alert` for blocking:

```bash
# Change alert rules to drop for high-confidence signatures
sed -i 's/^alert tcp.*4444/drop tcp/' /etc/suricata/rules/local.rules
sudo kill -USR2 $(pidof suricata)
```

**Warning:** Test NFQ mode in a lab before production. A high false-positive rate combined with `drop` rules causes service disruption.

---

## What you have built

- Suricata installed and configured with AF_PACKET for high-performance zero-copy capture
- `$HOME_NET` and `$EXTERNAL_NET` variables set to your network topology
- Emerging Threats Open ruleset loaded and updated with `suricata-update`
- EVE JSON output capturing alerts, HTTP, DNS, TLS, and flow metadata
- Custom rules for reverse shell detection, PowerShell cradles, and DNS tunneling
- False positive suppression with `threshold.conf` and `suricata-update` disable lists
- Alert forwarding to Elasticsearch/ELK via Filebeat and to Loki via Promtail
- NFQ inline IPS mode for active packet blocking
