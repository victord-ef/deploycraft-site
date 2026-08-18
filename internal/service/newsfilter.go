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

	title := strings.ToLower(article.Title)

	add := func(keyword string, points int) {
		if strings.Contains(title, keyword) {
			score += points
			if points >= 0 {
				reasons = append(reasons, fmt.Sprintf("+%d (%s)", points, keyword))
			} else {
				reasons = append(reasons, fmt.Sprintf("%d (%s)", points, keyword))
			}
		}
	}

	// Positive signals
	add("kubernetes", +30)
	add("cncf", +25)
	add("security", +30)
	add("vulnerability", +40)
	add("cve", +50)
	add("release", +25)
	add("graduation", +40)
	add("announces", +15)
	add("introducing", +20)
	add("launch", +20)
	add("openai", +20)
	add("aws", +10)

	// Negative signals
	add("weekly roundup", -70)
	add("newsletter", -50)
	add("webinar", -40)
	add("podcast", -40)
	add("football", -100)
	add("career", -60)

	return EvaluationResult{
		Article:    article,
		Score:      score,
		Newsworthy: score >= MinimumScore,
		Reasons:    reasons,
	}
}
