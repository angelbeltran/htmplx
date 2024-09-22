package htmx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvent_compileDetailsForTriggerHeader(t *testing.T) {
	type Expect struct {
		Result []byte
		AnErr  bool
	}

	type Test struct {
		Name   string
		Event  Event
		Expect Expect
	}

	tests := []Test{
		{
			Name: "no details results in null",
			Expect: Expect{
				Result: []byte(`null`),
			},
		},
		{
			Name: "no details with target results in object with target field",
			Event: Event{
				Target: "abc",
			},
			Expect: Expect{
				Result: []byte(`{"target":"#abc"}`),
			},
		},
		{
			Name: "details object results in details object json",
			Event: Event{
				Details: map[string]any{
					"key1": "value1",
				},
			},
			Expect: Expect{
				Result: []byte(`{"key1":"value1"}`),
			},
		},
		{
			Name: "details object with target results in details object with target field",
			Event: Event{
				Details: map[string]any{
					"key1": "value1",
				},
				Target: "abc",
			},
			Expect: Expect{
				Result: []byte(`{"key1":"value1","target":"#abc"}`),
			},
		},
		{
			Name: "details object with target prefixed with # results in details object with target field",
			Event: Event{
				Details: map[string]any{
					"key1": "value1",
				},
				Target: "#abc",
			},
			Expect: Expect{
				Result: []byte(`{"key1":"value1","target":"#abc"}`),
			},
		},
		{
			Name: "details string results in details object string",
			Event: Event{
				Details: "some string",
			},
			Expect: Expect{
				Result: []byte(`"some string"`),
			},
		},
		{
			Name: "details string with target results in error",
			Event: Event{
				Details: "some string",
				Target:  "abc",
			},
			Expect: Expect{
				AnErr: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			res, err := test.Event.compileDetailsForTriggerHeader()

			if test.Expect.AnErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.Expect.Result, res, "unexpected result")
		})
	}
}
