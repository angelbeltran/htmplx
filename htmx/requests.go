package htmx

// HTMX Request Headers
// https://htmx.org/docs/#request-headers

type RequestHeader = Header

func (h RequestHeader) IsHTMXRequest() bool {
	return h.Get("HX-Request") == "true"
}

// indicates that the request is via an element using hx-boost
func (h RequestHeader) IsBoosted() bool {
	_, boosted1 := h.Header["Hx-Boosted"]
	_, boosted2 := h.Header["HX-Boosted"]
	return boosted1 || boosted2
}

// “true” if the request is for history restoration after a miss in the local history cache
func (h RequestHeader) IsHistoryRestoreRequest() bool {
	return h.Get("HX-History-Restore-Request") == "true"
}

// the current URL of the browser
func (h RequestHeader) GetCurrentURL() string {
	return h.Get("HX-Current-URL")
}

// the id of the target element if it exists
func (h RequestHeader) GetTarget() string {
	return h.Get("HX-Target")
}

// the id of the triggered element if it exists
func (h RequestHeader) GetTrigger() string {
	return h.Get("HX-Trigger")
}

// the name of the triggered element if it exists
func (h RequestHeader) GetTriggerName() string {
	return h.Get("HX-Trigger-Name")
}

// the user response to an hx-prompt
func (h RequestHeader) GetPrompt() string {
	return h.Get("HX-Prompt")
}
