package service

import (
	"testing"

	"github.com/victord-ef/deploycraft-site/internal/models"
)

func TestEvaluate(t *testing.T) {
	tests := []struct {
		title      string
		source     string
		wantNews   bool
		minScore   int
	}{
		{"Critical Kubernetes security vulnerability fixed", "Kubernetes", true, 100},
		{"AWS Weekly Roundup", "AWS", false, 0},
		{"CNCF Announces Kubeflow Graduation", "CNCF", true, 80},
		{"Get closer to the game with Gemini", "Google AI", false, 0},
	}

	for _, tc := range tests {
		t.Run(tc.title, func(t *testing.T) {
			article := models.Article{Title: tc.title, Source: tc.source}
			result := Evaluate(article)

			if result.Newsworthy != tc.wantNews {
				t.Errorf("Newsworthy = %v, want %v (score %d, reasons %v)",
					result.Newsworthy, tc.wantNews, result.Score, result.Reasons)
			}
			if tc.wantNews && result.Score < tc.minScore {
				t.Errorf("Score = %d, want >= %d", result.Score, tc.minScore)
			}
		})
	}
}
