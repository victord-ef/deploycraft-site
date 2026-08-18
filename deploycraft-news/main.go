package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/victord-ef/deploycraft-site/internal/models"
	"github.com/victord-ef/deploycraft-site/internal/service"
	"github.com/victord-ef/deploycraft-site/internal/settings"
	"github.com/victord-ef/deploycraft-site/internal/storage"
)

func main() {
	db, err := storage.New("deploycraft-news/news.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		log.Fatal(err)
	}

	cmd := "ingest"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {

	case "ingest":
		cfg, err := settings.Load("config/configuration.yaml")
		if err != nil {
			log.Fatal(err)
		}
		if err := service.IngestFeeds(db, cfg); err != nil {
			log.Fatal(err)
		}
		total, err := db.Count()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Total articles in database: %d", total)

	case "latest":
		articles, err := service.Latest(db, 10)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%-6s  %-12s  %-12s  %-45s  %s\n", "ID", "STATUS", "SOURCE", "TITLE", "PUBLISHED")
		fmt.Println("-------  ------------  ------------  ---------------------------------------------  ----------")
		for _, a := range articles {
			title := a.Title
			if len(title) > 45 {
				title = title[:42] + "..."
			}
			source := a.Source
			if len(source) > 12 {
				source = source[:12]
			}
			fmt.Printf("%-6d  %-12s  %-12s  %-45s  %s\n",
				a.ID, a.Status, source, title, a.Published.Format("2006-01-02"))
		}

	case "queue":
		var articles []models.Article
		var err error
		if len(os.Args) > 2 {
			articles, err = service.FilteredQueue(db, os.Args[2])
		} else {
			articles, err = service.ReviewQueue(db)
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d articles waiting for review\n\n", len(articles))
		fmt.Printf("%-6s  %-12s  %-45s  %s\n", "ID", "SOURCE", "TITLE", "PUBLISHED")
		fmt.Println("-------  ------------  ---------------------------------------------  ----------")
		for _, a := range articles {
			title := a.Title
			if len(title) > 45 {
				title = title[:42] + "..."
			}
			source := a.Source
			if len(source) > 12 {
				source = source[:12]
			}
			fmt.Printf("%-6d  %-12s  %-45s  %s\n",
				a.ID, source, title, a.Published.Format("2006-01-02"))
		}

	case "publish":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: deploycraft-news publish <id> <rewrite-file>")
			os.Exit(1)
		}
		id, err := strconv.ParseInt(os.Args[2], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid ID: %s\n", os.Args[2])
			os.Exit(1)
		}
		if err := service.PublishArticle(db, id, os.Args[3], "content/news"); err != nil {
			log.Fatal(err)
		}

	case "purge":
		fmt.Print("Reject all NEW articles? This cannot be undone. (yes/no): ")
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "yes" {
			fmt.Println("Aborted.")
			os.Exit(0)
		}
		n, err := service.PurgeQueue(db)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d articles rejected.\n", n)

	case "select":
		id := requireID()
		if err := service.SelectArticle(db, id); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Article %d marked as SELECTED\n", id)

	case "reject":
		id := requireID()
		if err := service.RejectArticle(db, id); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Article %d marked as REJECTED\n", id)

	case "view":
		id := requireID()
		a, err := service.ReviewArticle(db, id)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("ID:          %d\n", a.ID)
		fmt.Printf("Status:      %s\n", a.Status)
		fmt.Printf("Title:       %s\n", a.Title)
		fmt.Printf("Source:      %s\n", a.Source)
		fmt.Printf("Published:   %s\n", a.Published.Format("2006-01-02 15:04"))
		fmt.Printf("Link:        %s\n", a.Link)
		fmt.Printf("\n%s\n", stripHTML(a.Description))
		if links := extractLinks(a.Description); len(links) > 0 {
			fmt.Println("\nLinks in article:")
			for _, l := range links {
				fmt.Printf("  %s\n", l)
			}
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}
}

func stripHTML(s string) string {
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func extractLinks(s string) []string {
	re := regexp.MustCompile(`href="(https?://[^"]+)"`)
	matches := re.FindAllStringSubmatch(s, -1)
	seen := map[string]bool{}
	var links []string
	for _, m := range matches {
		if !seen[m[1]] {
			links = append(links, m[1])
			seen[m[1]] = true
		}
	}
	return links
}

func requireID() int64 {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: deploycraft-news %s <id>\n", os.Args[1])
		os.Exit(1)
	}
	id, err := strconv.ParseInt(os.Args[2], 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid ID: %s\n", os.Args[2])
		os.Exit(1)
	}
	return id
}

func usage() {
	fmt.Println("Usage: deploycraft-news <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  ingest              Fetch RSS feeds and save newsworthy articles")
	fmt.Println("  latest              Show the 10 most recent articles")
	fmt.Println("  queue [source]      Show articles awaiting review (optional source filter)")
	fmt.Println("  view   <id>         Show full article details")
	fmt.Println("  select <id>         Mark article as selected for publication")
	fmt.Println("  reject <id>         Mark article as rejected")
	fmt.Println("  publish <id> <file>  Generate Hugo article from rewrite file")
	fmt.Println("  purge               Delete all NEW/REJECTED articles (start fresh)")
}
