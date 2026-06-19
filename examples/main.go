package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/kaptinlin/requests"
)

// Post represents an API post.
type Post struct {
	UserID int    `json:"userId"`
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

func main() {
	if err := runExample(); err != nil {
		log.Fatal(err)
	}
}

func runExample() error {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"userId":1,"id":1,"title":"hello","body":"world"}`))
	}))
	defer server.Close()

	return run(context.Background(), server.URL, "1", log.Default())
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
	client, err := requests.New(
		requests.WithBaseURL(baseURL),
		requests.WithTimeout(30*time.Second),
	)
	if err != nil {
		return Post{}, fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := client.Get("/posts/{post_id}").PathParam("post_id", postID).Send(ctx)
	if err != nil {
		return Post{}, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Close() }()

	var post Post
	if err := resp.DecodeJSON(&post); err != nil {
		return Post{}, fmt.Errorf("failed to parse response: %w", err)
	}

	return post, nil
}
