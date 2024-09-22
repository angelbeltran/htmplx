package htmx

import "net/http"

func singletonHeader(k, v string) http.Header {
	h := http.Header{}
	h.Set(k, v)
	return h
}
