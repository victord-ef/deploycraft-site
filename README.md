# DeployCraft.io

Source for [deploycraft.io](https://deploycraft.io) — a technical platform covering Kubernetes, CI/CD, GitOps, Infrastructure as Code, and DevSecOps. Written by a practitioner with nearly a decade across the full SDLC.

## Stack

- **Static site generator** — [Hugo](https://gohugo.io/) v0.133.0 extended
- **Hosting** — [Cloudflare Pages](https://pages.cloudflare.com/)
- **Source control** — GitHub (`main` is production, `dev` is the working branch)

## Project structure

```
.
├── content/            # Markdown content
│   ├── docs/           # Reference documentation by topic
│   ├── tutorials/      # Two-part tutorial series
│   ├── blog/
│   ├── labs/
│   ├── snippets/
│   ├── downloads/
│   └── about/
├── layouts/            # Hugo templates
│   ├── _default/       # base, single, list, taxonomy
│   ├── shortcodes/     # callout, codefile
│   ├── labs/
│   ├── snippets/
│   ├── downloads/
│   └── about/
├── static/
│   ├── css/style.css
│   ├── js/copy.js
│   └── images/
├── data/
│   └── topics.yaml     # Topic cards for homepage
└── hugo.toml           # Site configuration
```

## Local development

**Prerequisites:** Hugo v0.133.0 extended

```bash
hugo server -D --disableFastRender
```

Site runs at `http://localhost:1313`. The `-D` flag includes draft content. Use `--disableFastRender` when editing static files (CSS, JS, images) to ensure changes are picked up.

## Content

### Tutorials

Tutorials are structured as linked two-part series. Each post requires the following front matter:

```yaml
---
title: "Title"
description: "One-line description"
series: "Series Name"
part: 1
date: 2025-01-01
tags: [kubernetes, ci-cd]
toc: true
draft: false
---
```

### Shortcodes

**Callout boxes**

```
{{< callout type="note" title="Optional title" >}}
Content here. Supports **markdown**.
{{< /callout >}}
```

Types: `note`, `tip`, `warning`, `danger`

**Code block with filename**

```
{{< codefile lang="yaml" file="deployment.yaml" >}}
apiVersion: v1
kind: Pod
{{< /codefile >}}
```

## Deployment

Cloudflare Pages builds automatically from `main`. The build command is `hugo --minify`.

Branch protection on `main` requires PRs from `dev`. After merging, Cloudflare picks up the new commit and rebuilds.

## Content areas

| Section | Path | Status |
|---|---|---|
| Docs | `/docs/` | Active |
| Tutorials | `/tutorials/` | Active |
| Blog | `/blog/` | Planned |
| Labs | `/labs/` | Coming soon |
| Snippets | `/snippets/` | Coming soon |
| Downloads | `/downloads/` | Coming soon |
