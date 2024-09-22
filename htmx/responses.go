package htmx

import (
	"net/http"
)

// HTMX Response Headers
// https://htmx.org/docs/#response-headers

type ResponseHeader = Header

func NewResponseHeader() ResponseHeader {
	return ResponseHeader{
		Header: make(http.Header),
	}
}

// allows you to do a client-side redirect that does not do a full page reload
func (h ResponseHeader) Location(url string) Header {
	h.Set("HX-Location", url)
	return h
}

// pushes a new url into the history stack
func (h ResponseHeader) PushUrl(url string) Header {
	h.Set("HX-Push-Url", url)
	return h
}

// can be used to do a client-side redirect to a new location
func (h ResponseHeader) Redirect(url string) Header {
	h.Set("HX-Redirect", url)
	return h
}

// if set to “true” the client-side will do a full refresh of the page
func (h ResponseHeader) Refresh() Header {
	h.Set("HX-Refresh", "true")
	return h
}

// replaces the current URL in the location bar
func (h ResponseHeader) ReplaceUrl(url string) Header {
	h.Set("HX-Replace-Url", url)
	return h
}

// allows you to specify how the response will be swapped. See hx-swap for possible values
func (h ResponseHeader) Reswap(a HXSwapAttribute) Header {
	h.Set("HX-Reswap", a.BuildHXSwapExpression())
	return h
}

// a CSS selector that updates the target of the content update to a different element on the page
func (h ResponseHeader) Retarget(selector string) Header {
	h.Set("HX-Retarget", selector)
	return h
}

// a CSS selector that allows you to choose which part of the response is used to be swapped in. Overrides an existing hx-select on the triggering element
func (h ResponseHeader) Reselect(selector string) Header {
	h.Set("HX-Reselect", selector)
	return h
}

// allows you to trigger client-side events
func (h ResponseHeader) Trigger(es ...Event) (Header, error) {
	b, err := compileEvents(es)
	if err != nil {
		return h, err
	}

	h.Set("HX-Trigger", string(b))
	return h, nil
}

// allows you to trigger client-side events after the settle step
func (h ResponseHeader) TriggerAfterSettle(es ...Event) (Header, error) {
	b, err := compileEvents(es)
	if err != nil {
		return h, err
	}

	h.Set("HX-Trigger-After-Settle", string(b))
	return h, nil
}

// allows you to trigger client-side events after the swap step
func (h ResponseHeader) TriggerAfterSwap(es ...Event) (Header, error) {
	b, err := compileEvents(es)
	if err != nil {
		return h, err
	}

	h.Set("HX-Trigger-After-Swap", string(b))
	return h, nil
}
