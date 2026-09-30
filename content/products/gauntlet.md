---
title: "Gauntlet"
description: "AI-powered security scanner that runs your repository through eight specialist tools in parallel, then uses Claude to triage findings, cut false positives, and deliver a prioritised remediation report."
icon: "⚔️"
weight: 2
cta_title: "Book a Gauntlet scan"
cta_body: "Send us your repository and we run the full suite — scanners, AI triage, and a PDF report — delivered within 24 hours. Flat-fee, no scope creep."
cta_btn: "Request a scan →"
pricing:
  - tier: "Snapshot"
    price: "$500"
    price_suffix: "flat"
    tagline: "One-off automated audit."
    featured: false
    includes:
      - "Full 8-scanner parallel run"
      - "Claude AI triage (false positive removal)"
      - "P0–P3 prioritised findings"
      - "Markdown + PDF report"
      - "Delivered within 24 hours"
  - tier: "Scan + Review"
    price: "$1,200"
    price_suffix: "flat"
    tagline: "Automated scan with practitioner review."
    featured: true
    includes:
      - "Everything in Snapshot"
      - "Manual validation of critical findings"
      - "Remediation code examples"
      - "60-min findings walkthrough call"
      - "One re-scan after fixes applied"
  - tier: "Continuous"
    price: "$800"
    price_suffix: "/mo"
    tagline: "Scheduled scanning on every release."
    featured: false
    includes:
      - "Weekly automated Gauntlet runs"
      - "Delta report (new findings only)"
      - "Slack alert on CRITICAL/HIGH findings"
      - "Monthly trend report"
      - "Async support channel"
services:
  - name: "SAST — Static Analysis"
    icon: "🔬"
    description: "Semgrep scans across 20+ languages against OWASP patterns, injection flaws, insecure deserialization, broken access control, and hundreds of custom security rules. Findings include the exact line, surrounding context, and a suggested fix."
  - name: "SCA — Dependency CVEs"
    icon: "📦"
    description: "Snyk checks every dependency in your project — npm, pip, Maven, Go modules — against the NVD and Snyk vulnerability database. Each CVE is reported with CVSS score, affected version range, and the exact upgrade that resolves it."
  - name: "Secret Detection"
    icon: "🔑"
    description: "Trufflehog and Gitleaks run in parallel — Trufflehog performs entropy analysis and verifies secrets against live APIs (AWS, GitHub, Stripe, and 800+ others), while Gitleaks sweeps commits, branches, and stash for hardcoded credentials and tokens."
  - name: "IaC Security"
    icon: "🏗️"
    description: "Checkov audits Terraform modules, Helm charts, and Kubernetes YAML manifests against 1,000+ security and compliance policies. Catches misconfigured storage buckets, overly permissive IAM roles, missing network policies, and containers running as root before they reach production."
  - name: "Kubernetes Compliance"
    icon: "☸️"
    description: "Kubescape maps your manifests or live cluster against the NSA/CISA Kubernetes hardening framework and MITRE ATT&CK for containers. kube-bench validates live nodes against the CIS Kubernetes Benchmark, checking API server flags, etcd encryption, kubelet configuration, and RBAC posture."
  - name: "Cloud Security Posture"
    icon: "☁️"
    description: "Prowler runs 300+ checks across AWS, GCP, and Azure — covering IAM least privilege, S3 bucket exposure, CloudTrail gaps, security group misconfigurations, encryption at rest, MFA enforcement, and SOC2/CIS/GDPR compliance controls."
  - name: "AI Triage"
    icon: "🤖"
    description: "Claude (claude-opus-4-7, adaptive thinking) reads flagged files and searches the codebase before deciding. Common noise — test fixtures with dummy credentials, vendor code, SAST rules firing on safe patterns — is marked false positive with a reason. Every true positive gets a specific, actionable remediation written by an AI that has read the relevant code."
  - name: "Prioritised Report"
    icon: "📋"
    description: "Findings are bucketed P0 (fix immediately) through P3 (backlog) with an overall risk score, executive summary, and a false-positives appendix. Delivered as markdown and a styled PDF — ready to share with your engineering team or security team without editing."
---

Most automated security scanners produce hundreds of findings and leave you to figure out what matters. Gauntlet is different: it runs eight specialist tools in parallel, then passes every finding to a Claude AI agent that reads the actual code before deciding. False positives are caught. Real vulnerabilities get specific fixes, not generic advice.

The result is a report your engineers can act on the same day it arrives.

Gauntlet is also open source. Run it yourself against any repository — or let us run it for you and deliver a report within 24 hours.

[View on GitHub →](https://github.com/victord-ef/Gauntlet)
