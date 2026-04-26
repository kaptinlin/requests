package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kaptinlin/requests"
)

// Post represents a JSONPlaceholder post.
type Post struct {
	UserID int    `json:"userId"`
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

func main() {
	if err := run(context.Background(), "https://jsonplaceholder.typicode.com", "1", log.Default()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, baseURL, postID string, logger *log.Logger) error {
	post, err := fetchPost(ctx, baseURL, postID)
	if err != nil {
		return err
	}

	logger.Printf("Post Received: %+v\n", post)
	return nil
}

func fetchPost(ctx context.Context, baseURL, postID string) (Post, error) {
	client := requests.Create(&requests.Config{
		BaseURL: baseURL,
		Timeout: 30 * time.Second,
	})

	resp, err := client.Get("/posts/{post_id}").PathParam("post_id", postID).Send(ctx)
	if err != nil {
		return Post{}, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Close() }()

	var post Post
	if err := resp.ScanJSON(&post); err != nil {
		return Post{}, fmt.Errorf("failed to parse response: %w", err)
	}

	return post, nil
}
