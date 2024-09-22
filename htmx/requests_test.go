package htmx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestHeader_IsHTMXRequest(t *testing.T) {
	type Test struct {
		Name          string
		Header        RequestHeader
		IsHTMXRequest bool
	}

	tests := []Test{
		{
			Name:          "true",
			Header:        singletonRequestHeader("HX-Request", "true"),
			IsHTMXRequest: true,
		},
		{
			Name:   "false",
			Header: singletonRequestHeader("HX-Request", "false"),
		},
		{
			Name:   "abc",
			Header: singletonRequestHeader("HX-Request", "abc"),
		},
		{
			Name:   "<empty>",
			Header: singletonRequestHeader("HX-Request", ""),
		},
		{
			Name: "<no header>",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.IsHTMXRequest, test.Header.IsHTMXRequest())
		})
	}
}

func TestRequestHeader_IsBoosted(t *testing.T) {
	type Test struct {
		Name      string
		Header    RequestHeader
		IsBoosted bool
	}

	tests := []Test{
		{
			Name:      "true",
			Header:    singletonRequestHeader("HX-Boosted", "true"),
			IsBoosted: true,
		},
		{
			Name:      "false",
			Header:    singletonRequestHeader("HX-Boosted", "false"),
			IsBoosted: true,
		},
		{
			Name:      "abc",
			Header:    singletonRequestHeader("HX-Boosted", "abc"),
			IsBoosted: true,
		},
		{
			Name:      "<empty>",
			Header:    singletonRequestHeader("HX-Boosted", ""),
			IsBoosted: true,
		},
		{
			Name: "<no header>",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.IsBoosted, test.Header.IsBoosted())
		})
	}
}

func TestRequestHeader_IsHistoryRestoreRequest(t *testing.T) {
	type Test struct {
		Name                    string
		Header                  RequestHeader
		IsHistoryRestoreRequest bool
	}

	tests := []Test{
		{
			Name:                    "true",
			Header:                  singletonRequestHeader("HX-History-Restore-Request", "true"),
			IsHistoryRestoreRequest: true,
		},
		{
			Name:   "false",
			Header: singletonRequestHeader("HX-History-Restore-Request", "false"),
		},
		{
			Name:   "abc",
			Header: singletonRequestHeader("HX-History-Restore-Request", "abc"),
		},
		{
			Name:   "<empty>",
			Header: singletonRequestHeader("HX-History-Restore-Request", ""),
		},
		{
			Name: "<no header>",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.IsHistoryRestoreRequest, test.Header.IsHistoryRestoreRequest())
		})
	}
}

func TestRequestHeader_GetCurrentURL(t *testing.T) {
	type Test struct {
		Name       string
		Header     RequestHeader
		CurrentURL string
	}

	tests := []Test{
		{
			Name:       "abc",
			Header:     singletonRequestHeader("HX-Current-URL", "abc"),
			CurrentURL: "abc",
		},
		{
			Name:       "xyz",
			Header:     singletonRequestHeader("HX-Current-URL", "xyz"),
			CurrentURL: "xyz",
		},
		{
			Name: "<no header>",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.CurrentURL, test.Header.GetCurrentURL())
		})
	}
}

func TestRequestHeader_GetTarget(t *testing.T) {
	type Test struct {
		Name   string
		Header RequestHeader
		Target string
	}

	tests := []Test{
		{
			Name:   "abc",
			Header: singletonRequestHeader("HX-Target", "abc"),
			Target: "abc",
		},
		{
			Name:   "xyz",
			Header: singletonRequestHeader("HX-Target", "xyz"),
			Target: "xyz",
		},
		{
			Name: "<no header>",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.Target, test.Header.GetTarget())
		})
	}
}

func TestRequestHeader_GetTrigger(t *testing.T) {
	type Test struct {
		Name    string
		Header  RequestHeader
		Trigger string
	}

	tests := []Test{
		{
			Name:    "abc",
			Header:  singletonRequestHeader("HX-Trigger", "abc"),
			Trigger: "abc",
		},
		{
			Name:    "xyz",
			Header:  singletonRequestHeader("HX-Trigger", "xyz"),
			Trigger: "xyz",
		},
		{
			Name: "<header>",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.Trigger, test.Header.GetTrigger())
		})
	}
}

func TestRequestHeader_GetTriggerName(t *testing.T) {
	type Test struct {
		Name        string
		Header      RequestHeader
		TriggerName string
	}

	tests := []Test{
		{
			Name:        "abc",
			Header:      singletonRequestHeader("HX-Trigger-Name", "abc"),
			TriggerName: "abc",
		},
		{
			Name:        "xyz",
			Header:      singletonRequestHeader("HX-Trigger-Name", "xyz"),
			TriggerName: "xyz",
		},
		{
			Name: "<no header>",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.TriggerName, test.Header.GetTriggerName())
		})
	}
}

func TestRequestHeader_GetPrompt(t *testing.T) {
	type Test struct {
		Name   string
		Header RequestHeader
		Prompt string
	}

	tests := []Test{
		{
			Name:   "abc",
			Header: singletonRequestHeader("HX-Prompt", "abc"),
			Prompt: "abc",
		},
		{
			Name:   "xyz",
			Header: singletonRequestHeader("HX-Prompt", "xyz"),
			Prompt: "xyz",
		},
		{
			Name: "<no header>",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.Prompt, test.Header.GetPrompt())
		})
	}
}

func singletonRequestHeader(k, v string) RequestHeader {
	return RequestHeader{singletonHeader(k, v)}
}
