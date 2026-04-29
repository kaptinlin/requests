package requests

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/kaptinlin/orderedobject"
)

type orderedHeadersContextKey struct{}

func cloneOrderedHeaders(headers *orderedobject.Object[[]string]) *orderedobject.Object[[]string] {
	if headers == nil {
		return nil
	}

	clone := orderedobject.NewObject[[]string](headers.Len())
	headers.ForEach(func(key string, values []string) {
		clone.Set(key, slices.Clone(values))
	})
	return clone
}

func isPseudoHeader(name string) bool {
	return strings.HasPrefix(name, ":")
}

func orderedHeaderKey(headers *orderedobject.Object[[]string], key string) (string, bool) {
	if headers == nil {
		return "", false
	}
	if headers.Has(key) {
		return key, true
	}
	for _, entry := range headers.Entries() {
		if strings.EqualFold(entry.Key, key) {
			return entry.Key, true
		}
	}
	return "", false
}

func headerFromOrderedHeaders(headers *orderedobject.Object[[]string]) http.Header {
	dst := http.Header{}
	addOrderedHeaders(dst, headers)
	return dst
}

func addOrderedHeaders(dst http.Header, headers *orderedobject.Object[[]string]) {
	if headers == nil {
		return
	}

	headers.ForEach(func(key string, values []string) {
		if isPseudoHeader(key) {
			return
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	})
}

func addHeaderValues(dst http.Header, src http.Header, ordered *orderedobject.Object[[]string]) {
	if ordered != nil {
		addOrderedHeaders(dst, ordered)
	}
	for key, values := range src {
		if _, ok := orderedHeaderKey(ordered, key); ok {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func overlayHeaderValues(dst http.Header, src http.Header, ordered *orderedobject.Object[[]string]) {
	for key := range src {
		deleteHeaderValues(dst, key)
	}
	if ordered != nil {
		ordered.ForEach(func(key string, _ []string) {
			deleteHeaderValues(dst, key)
		})
	}
	addHeaderValues(dst, src, ordered)
}

func deleteHeaderValues(headers http.Header, key string) {
	headers.Del(key)
	for existing := range headers {
		if strings.EqualFold(existing, key) {
			delete(headers, existing)
		}
	}
}

func setOrderedHeaderValues(headers **orderedobject.Object[[]string], key string, values []string) {
	if *headers == nil {
		*headers = orderedobject.NewObject[[]string]()
	}
	if existing, ok := orderedHeaderKey(*headers, key); ok {
		key = existing
	}
	(*headers).Set(key, slices.Clone(values))
}

func addOrderedHeaderValue(headers **orderedobject.Object[[]string], key, value string) {
	if *headers == nil {
		*headers = orderedobject.NewObject[[]string]()
	}

	existing, ok := orderedHeaderKey(*headers, key)
	if ok {
		key = existing
	}
	values, ok := (*headers).Get(key)
	if ok {
		values = append(slices.Clone(values), value)
		(*headers).Set(key, values)
		return
	}
	(*headers).Set(key, []string{value})
}

func deleteOrderedHeader(headers *orderedobject.Object[[]string], key string) {
	if headers == nil {
		return
	}
	if existing, ok := orderedHeaderKey(headers, key); ok {
		headers.Delete(existing)
	}
}

func mergeOrderedHeaders(
	base *orderedobject.Object[[]string],
	override *orderedobject.Object[[]string],
) *orderedobject.Object[[]string] {
	merged := cloneOrderedHeaders(base)
	if override == nil {
		return merged
	}
	if merged == nil {
		return cloneOrderedHeaders(override)
	}

	override.ForEach(func(key string, values []string) {
		deleteOrderedHeader(merged, key)
		merged.Set(key, slices.Clone(values))
	})
	return merged
}

func withOrderedHeaders(req *http.Request, headers *orderedobject.Object[[]string]) *http.Request {
	if headers == nil {
		return req
	}
	ctx := context.WithValue(req.Context(), orderedHeadersContextKey{}, cloneOrderedHeaders(headers))
	return req.WithContext(ctx)
}

// OrderedHeaders returns the ordered header metadata attached to req, when present.
//
// Default net/http transports do not guarantee wire-order delivery. This metadata is
// intended for transports that explicitly support ordered headers.
func OrderedHeaders(req *http.Request) (*orderedobject.Object[[]string], bool) {
	if req == nil {
		return nil, false
	}
	headers, ok := req.Context().Value(orderedHeadersContextKey{}).(*orderedobject.Object[[]string])
	if !ok || headers == nil {
		return nil, false
	}
	return cloneOrderedHeaders(headers), true
}
