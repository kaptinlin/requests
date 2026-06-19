package requests_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/kaptinlin/requests"
)

func ExampleClient_Get() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":1,"title":"delectus aut autem"}`)
	}))
	defer server.Close()

	type post struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}

	client, err := requests.New(
		requests.WithBaseURL(server.URL),
		requests.WithTimeout(5*time.Second),
	)
	if err != nil {
		fmt.Println("client error:", err)
		return
	}

	resp, err := client.Get("/posts/{id}").PathParam("id", "1").Send(context.Background())
	if err != nil {
		fmt.Println("request error:", err)
		return
	}
	defer func() { _ = resp.Close() }()

	var p post
	if err := resp.DecodeJSON(&p); err != nil {
		fmt.Println("decode error:", err)
		return
	}

	fmt.Println(p.ID, p.Title)
	// Output:
	// 1 delectus aut autem
}
