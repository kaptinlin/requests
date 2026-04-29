package browser

import (
	"fmt"

	"github.com/kaptinlin/orderedobject"
	"github.com/kaptinlin/requests"
)

type profile struct {
	name    string
	headers *orderedobject.Object[[]string]
	http2   bool
}

// Chrome returns a Chrome identity profile.
func Chrome() requests.Profile {
	return profile{
		name:    "Chrome",
		headers: chromeHeaders(),
		http2:   true,
	}
}

// Firefox returns a Firefox identity profile.
func Firefox() requests.Profile {
	return profile{
		name:    "Firefox",
		headers: firefoxHeaders(),
		http2:   true,
	}
}

func (p profile) Name() string {
	return p.name
}

func (p profile) Apply(c *requests.Client) error {
	if c == nil {
		return fmt.Errorf("%w: client", requests.ErrInvalidConfigValue)
	}
	c.SetDefaultOrderedHeaders(p.headers)
	if p.http2 {
		c.EnableHTTP2()
	}
	return nil
}

func chromeHeaders() *orderedobject.Object[[]string] {
	return baseHeaders("en-US,en;q=0.9").
		Set("Sec-CH-UA", []string{`"Not:A-Brand";v="99", "Google Chrome";v="145", "Chromium";v="145"`}).
		Set("Sec-CH-UA-Mobile", []string{"?0"}).
		Set("Sec-CH-UA-Platform", []string{`"Windows"`}).
		Set("User-Agent", []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36"})
}

func firefoxHeaders() *orderedobject.Object[[]string] {
	return baseHeaders("en-US,en;q=0.5").
		Set("User-Agent", []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:148.0) Gecko/20100101 Firefox/148.0"})
}

func baseHeaders(acceptLanguage string) *orderedobject.Object[[]string] {
	return orderedobject.NewObject[[]string]().
		Set(":authority", nil).
		Set(":method", nil).
		Set(":path", nil).
		Set(":scheme", nil).
		Set("Accept-Encoding", []string{"gzip, deflate"}).
		Set("Accept-Language", []string{acceptLanguage})
}
