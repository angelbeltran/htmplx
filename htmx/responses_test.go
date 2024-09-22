package htmx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResponseHeader_Location(t *testing.T) {
	h := NewResponseHeader()
	h.Location("ABC")
	assert.Equal(t, h.Get("HX-Location"), "ABC")
}

func TestResponseHeader_PushUrl(t *testing.T) {
	h := NewResponseHeader()
	h.PushUrl("ABC")
	assert.Equal(t, h.Get("HX-Push-Url"), "ABC")
}

func TestResponseHeader_Redirect(t *testing.T) {
	h := NewResponseHeader()
	h.Redirect("ABC")
	assert.Equal(t, h.Get("HX-Redirect"), "ABC")
}

func TestResponseHeader_Refresh(t *testing.T) {
	h := NewResponseHeader()
	h.Refresh()
	assert.Equal(t, h.Get("HX-Refresh"), "true")
}

func TestResponseHeader_ReplaceUrl(t *testing.T) {
	h := NewResponseHeader()
	h.ReplaceUrl("ABC")
	assert.Equal(t, h.Get("HX-Replace-Url"), "ABC")
}

func TestResponseHeader_Reswap(t *testing.T) {
	h := NewResponseHeader()
	h.Reswap(HXSwapInnerHTML)
	assert.Equal(t, h.Get("HX-Reswap"), "innerHTML")
}

func TestResponseHeader_Retarget(t *testing.T) {
	h := NewResponseHeader()
	h.Retarget("ABC")
	assert.Equal(t, h.Get("HX-Retarget"), "ABC")
}

func TestResponseHeader_Reselect(t *testing.T) {
	h := NewResponseHeader()
	h.Reselect("ABC")
	assert.Equal(t, h.Get("HX-Reselect"), "ABC")
}

func TestResponseHeader_Trigger(t *testing.T) {
	h := NewResponseHeader()
	_, err := h.Trigger(Event{
		Name: "ABC",
	})
	assert.NoError(t, err)
	assert.Equal(t, `{"ABC":null}`, h.Get("HX-Trigger"))
}

func TestResponseHeader_TriggerAfterSettle(t *testing.T) {
	h := NewResponseHeader()
	_, err := h.TriggerAfterSettle(Event{
		Name: "ABC",
	})
	assert.NoError(t, err)
	assert.Equal(t, `{"ABC":null}`, h.Get("HX-Trigger-After-Settle"))
}

func TestResponseHeader_TriggerAfterSwap(t *testing.T) {
	h := NewResponseHeader()
	_, err := h.TriggerAfterSwap(Event{
		Name: "ABC",
	})
	assert.NoError(t, err)
	assert.Equal(t, `{"ABC":null}`, h.Get("HX-Trigger-After-Swap"))
}
