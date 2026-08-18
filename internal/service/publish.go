package service

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/victord-ef/deploycraft-site/internal/models"
	"github.com/victord-ef/deploycraft-site/internal/storage"
)

func PublishArticle(db *storage.SQLite, id int64, rewriteFile string, outputDir string, noAttribution bool) error {
	article, err := db.FindByID(id)
	if err != nil {
		return fmt.Errorf("article not found: %w", err)
	}

	raw, err := os.ReadFile(rewriteFile)
	if err != nil {
		return fmt.Errorf("reading rewrite file: %w", err)
	}

	title, body := parseRewrite(string(raw))
	if title == "" {
		return fmt.Errorf("rewrite file must have a title on the first line")
	}

	slug := slugify(title)
	seoDesc := seoDescription(body)
	if article.Description != "" {
		seoDesc = stripHTMLDesc(article.Description)
	}
	tags := buildTags(article)

	md := renderMarkdown(title, slug, seoDesc, tags, body, article, noAttribution)

	filename := filepath.Join(outputDir, slug+".md")
	if err := os.WriteFile(filename, []byte(md), 0644); err != nil {
		return fmt.Errorf("writing markdown: %w", err)
	}

	if err := db.UpdateSummary(id, title, body, strings.Join(tags, ", "), seoDesc); err != nil {
		return fmt.Errorf("saving summary: %w", err)
	}
	if err := db.UpdateStatus(id, models.StatusPublished); err != nil {
		return fmt.Errorf("updating status: %w", err)
	}

	fmt.Printf("Published: %s\n", filename)
	return nil
}

// parseRewrite splits the rewrite file into title (first line) and body (rest).
// Strips leading # from the title if present.
func parseRewrite(content string) (title, body string) {
	content = strings.TrimSpace(content)
	idx := strings.Index(content, "\n")
	if idx == -1 {
		return strings.TrimPrefix(strings.TrimSpace(content), "# "), ""
	}
	title = strings.TrimPrefix(strings.TrimSpace(content[:idx]), "# ")
	body = strings.TrimSpace(content[idx+1:])
	return
}

func slugify(title string) string {
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s := strings.ToLower(title)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = s[:80]
		s = strings.TrimRight(s, "-")
	}
	return s
}

// seoDescription takes the first ~155 chars of the body as the meta description.
func seoDescription(body string) string {
	plain := regexp.MustCompile(`[#*_\[\]` + "`" + `]`).ReplaceAllString(body, "")
	plain = strings.Join(strings.Fields(plain), " ")
	if len(plain) <= 155 {
		return plain
	}
	cut := plain[:155]
	if i := strings.LastIndex(cut, " "); i > 100 {
		cut = cut[:i]
	}
	return cut + "..."
}

// buildTags derives tags from the article source and title keywords.
func buildTags(article *models.Article) []string {
	tagMap := map[string]string{
		"kubernetes":    "kubernetes",
		"cncf":          "cncf",
		"helm":          "helm",
		"argocd":        "argocd",
		"argo-cd":       "argocd",
		"flux":          "flux",
		"cilium":        "cilium",
		"istio":         "istio",
		"falco":         "falco",
		"kyverno":       "kyverno",
		"trivy":         "trivy",
		"vault":         "vault",
		"security":      "security",
		"vulnerability": "vulnerability",
		"cve":           "cve",
		"ransomware":    "ransomware",
		"exploit":       "exploit",
		"zero-day":      "zero-day",
		"patch":         "patch",
		"aws":           "aws",
		"terraform":     "terraform",
		"docker":        "docker",
		"containerd":    "containerd",
	}

	seen := map[string]bool{}
	var tags []string

	text := strings.ToLower(article.Title + " " + article.Source)
	for keyword, tag := range tagMap {
		if strings.Contains(text, keyword) && !seen[tag] {
			tags = append(tags, tag)
			seen[tag] = true
		}
	}

	tags = append(tags, "news")
	if !seen["security"] {
		tags = append(tags, "devsecops")
	}
	return tags
}

func stripHTMLDesc(s string) string {
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 155 {
		cut := s[:155]
		if i := strings.LastIndex(cut, " "); i > 100 {
			cut = cut[:i]
		}
		s = cut + "..."
	}
	return s
}

func renderMarkdown(title, slug, seoDesc string, tags []string, body string, article *models.Article, noAttribution bool) string {
	tagList := `["` + strings.Join(tags, `", "`) + `"]`
	date := time.Now().Format("2006-01-02")

	var footer string
	if !noAttribution {
		footer = fmt.Sprintf(
			"\n\n---\n*Originally reported by [%s](%s). Editorial coverage by DeployCraft.*",
			article.Source, article.Link,
		)
	}

	return fmt.Sprintf(`---
title: %q
date: %s
author: "Victor D"
description: %q
tags: %s
categories: ["news"]
draft: false
toc: true
source: %q
source_url: %q
---

%s%s
`, title, date, seoDesc, tagList, article.Source, article.Link, body, footer)
}
