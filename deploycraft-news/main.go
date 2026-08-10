package main

import (
	"fmt"
	"log"

	"github.com/victord-ef/RSS/internal/config"
)

func main() {

	cfg, err := settings.Load("../config/configuration.yaml")
	if err != nil {
		log.Fatal(err)
	}

	for _, feed := range cfg.Feeds {
		fmt.Println(feed.Name)
		fmt.Println(feed.URL)
	}
}
