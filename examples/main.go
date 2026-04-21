package main

import (
	"context"
	"log"
	"time"

	"github.com/kaptinlin/requests"
)

// Post represents a JSONPlaceholder post.
type Post struct {
	UserID int    `json:"userId"` // UserID is the post author's identifier.
	ID     int    `json:"id"`     // ID is the post identifier.
	Title  string `json:"title"`  // Title is the post title.
	Body   string `json:"body"`   // Body is the post content.
}

func main() {
	client := requests.Create(&requests.Config{
		BaseURL: "https://jsonplaceholder.typicode.com",
		Timeout: 30 * time.Second,
	})

	resp, err := client.Get("/posts/{post_id}").PathParam("post_id", "1").Send(context.Background())
	if err != nil {
		log.Fatalf("Failed to make request: %v", err)
	}

	var post Post
	if err := resp.ScanJSON(&post); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	log.Printf("Post Received: %+v\n", post)
}
