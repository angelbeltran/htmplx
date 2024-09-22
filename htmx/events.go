package htmx

import (
	"encoding/json"
	"fmt"
	"strings"
)

type (
	// Event is for triggering client-side events.
	Event struct {
		Name string
		// Details are optional.
		Details JSON
		// Target optionally can specify a target element.
		Target HTMLID
	}

	JSON   = any
	HTMLID = string
)

var (
	errNotAnJSONObject = fmt.Errorf("not a json object")
)

func compileEvents(es []Event) ([]byte, error) {
	m := make(map[string]any, len(es))
	for _, e := range es {
		details, err := e.compileDetailsForTriggerHeader()
		if err != nil {
			return nil, fmt.Errorf("unable to marshal event details to json: %w", err)
		}

		m[e.Name] = json.RawMessage(details)
	}

	return json.Marshal(m)
}

// compileDetailsForTriggerHeader will return a json null if no details are specified.
func (e Event) compileDetailsForTriggerHeader() ([]byte, error) {
	var details []byte
	if e.Details != nil {
		var err error
		if details, err = json.Marshal(e.Details); err != nil {
			return nil, err
		}
	}

	if e.Target == "" {
		if details == nil {
			details = []byte("null")
		}
		return details, nil
	}

	if details == nil || string(details) == "null" {
		details = []byte("{}")
	}
	if len(details) < 2 || details[0] != '{' {
		return nil, fmt.Errorf("cannot add target to event details: %w", errNotAnJSONObject)
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(details, &obj); err != nil {
		return nil, fmt.Errorf("failed to add target to event details: unble to unmarshal event details into a map: %w", err)
	}

	if obj == nil {
		obj = make(map[string]json.RawMessage, 1)
	}

	obj["target"] = json.RawMessage(fmt.Sprintf("%q", "#"+strings.TrimPrefix(e.Target, "#")))

	var err error
	if details, err = json.Marshal(obj); err != nil {
		return nil, fmt.Errorf("failed to marshal details with added target back to json: %w", err)
	}

	return details, nil
}
