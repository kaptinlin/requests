package requests

import "net/http"

func headerCookies(headers http.Header) []*http.Cookie {
	return (&http.Request{Header: headers}).Cookies()
}

func applyCookiePrecedence(req *http.Request, defaults, overrides []*http.Cookie) {
	cookies := make([]*http.Cookie, 0, len(defaults)+len(overrides))
	positions := make(map[string]int, cap(cookies))
	for _, layer := range [][]*http.Cookie{defaults, overrides} {
		for _, cookie := range layer {
			if cookie == nil {
				continue
			}
			if position, ok := positions[cookie.Name]; ok {
				cookies[position] = cookie
				continue
			}
			positions[cookie.Name] = len(cookies)
			cookies = append(cookies, cookie)
		}
	}

	req.Header.Del("Cookie")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
}
