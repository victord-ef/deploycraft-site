package service

import (
	"fmt"
	"strings"

	"github.com/victord-ef/deploycraft-site/internal/models"
)

const MinimumScore = 70

type EvaluationResult struct {
	Article    models.Article
	Score      int
	Newsworthy bool
	Reasons    []string
}

func Evaluate(article models.Article) EvaluationResult {
	score := 0
	var reasons []string

	// Score against title + source so GitHub release items (title = "v1.31.0")
	// are matched via their feed source name (e.g. "Kubernetes Releases").
	text := strings.ToLower(article.Title + " " + article.Source)

	add := func(keyword string, points int) {
		if strings.Contains(text, keyword) {
			score += points
			if points >= 0 {
				reasons = append(reasons, fmt.Sprintf("+%d (%s)", points, keyword))
			} else {
				reasons = append(reasons, fmt.Sprintf("%d (%s)", points, keyword))
			}
		}
	}

	// ── Platform & orchestration ──────────────────────────────────────────
	add("kubernetes", +30)
	add("cncf", +25)
	add("helm", +25)
	add("argocd", +25)
	add("argo-cd", +25)
	add("argo cd", +25)
	add("flux", +20)
	add("cilium", +30)
	add("istio", +25)
	add("linkerd", +25)
	add("containerd", +25)
	add("keda", +20)

	// ── Security tooling ─────────────────────────────────────────────────
	add("falco", +30)
	add("kyverno", +30)
	add("trivy", +25)
	add("cert-manager", +25)
	add("vault", +20)

	// ── Security signals ─────────────────────────────────────────────────
	add("security", +30)
	add("vulnerability", +40)
	add("cve", +50)
	add("patch", +25)
	add("advisory", +35)
	add("exploit", +40)
	add("breach", +35)
	add("malware", +30)
	add("ransomware", +35)
	add("zero-day", +50)
	add("zero day", +50)

	// ── Release & announcement signals ───────────────────────────────────
	add("release", +25)
	add("graduation", +40)
	add("announces", +15)
	add("introducing", +20)
	add("launch", +20)

	// ── Cloud providers ──────────────────────────────────────────────────
	add("aws", +10)
	add("openai", +10)

	// ── Noise ────────────────────────────────────────────────────────────
	add("weekly roundup", -70)
	add("newsletter", -50)
	add("webinar", -40)
	add("podcast", -40)
	add("football", -100)
	add("career", -60)
	add("hiring", -60)
	add("job", -40)
	add("sponsor", -50)

	return EvaluationResult{
		Article:    article,
		Score:      score,
		Newsworthy: score >= MinimumScore,
		Reasons:    reasons,
	}
}
