package htmx

import (
	"fmt"
	"strings"
	"time"
)

// https://htmx.org/docs/#swapping
// https://htmx.org/attributes/hx-swap/

type (
	HXSwapAttribute interface {
		BuildHXSwapExpression() string
	}

	// HXSwapOption is a HXSwapAttribute
	HXSwapOption string

	// ModifiedHXSwapOption is a HXSwapAttribute
	ModifiedHXSwapOption struct {
		Option    HXSwapOption
		Modifiers []HXSwapModifier
	}

	HXSwapModifier interface {
		BuildModifierExpression() string
	}

	// UseViewTransitionAPI is a HXSwapModifier
	// true or false, whether to use the view transition API for this swap
	UseViewTransitionAPI bool

	// SwapDelay is a HXSwapModifier
	// The swap delay to use (e.g. 100ms) between when old content is cleared and the new content is inserted
	SwapDelay time.Duration

	// SettleDelay is a HXSwapModifier
	// The settle delay to use (e.g. 100ms) between when new content is inserted and when it is settled
	SettleDelay time.Duration

	// IgnoreTitle is a HXSwapModifier
	// If set to true, any title found in the new content will be ignored and not update the document title
	IgnoreTitle bool

	// ScrollTo is a HXSwapModifier
	// top or bottom, will scroll the target element to its top or bottom
	ScrollTo struct {
		Side     TopOrBottom
		Selector string
	}

	// Show is a HXSwapModifier
	// top or bottom, will scroll the target element’s top or bottom into view
	Show struct {
		Side     TopOrBottom
		Selector string
	}

	// FocusScroll is a HXSwapModifier
	FocusScroll bool

	// TopOrBottom describes the direction of the swap modifier
	TopOrBottom bool
)

const (
	// the default, puts the content inside the target element
	HXSwapInnerHTML HXSwapOption = "innerHTML"
	// replaces the entire target element with the returned content
	HXSwapOuterHTML HXSwapOption = "outerHTML"
	// prepends the content before the first child inside the target
	HXSwapAfterbegin HXSwapOption = "afterbegin"
	// prepends the content before the target in the target’s parent element
	HXSwapBeforebegin HXSwapOption = "beforebegin"
	// appends the content after the last child inside the target
	HXSwapBeforeend HXSwapOption = "beforeend"
	// appends the content after the target in the target’s parent element
	HXSwapAfterend HXSwapOption = "afterend"
	// deletes the target element regardless of the response
	HXSwapDelete HXSwapOption = "delete"
	// does not append content from response (Out of Band Swaps and Response Headers will still be processed)
	HXSwapNone HXSwapOption = "none"

	Top    TopOrBottom = false
	Bottom TopOrBottom = true

	// WindowSelector may be used as Selector with ScrollTo or Show
	WindowSelector = "window"
)

func (o HXSwapOption) BuildHXSwapExpression() string {
	return string(o)
}

func (o HXSwapOption) Modify(m HXSwapModifier) ModifiedHXSwapOption {
	return ModifiedHXSwapOption{
		Option: o,
		Modifiers: []HXSwapModifier{
			m,
		},
	}
}

func (o HXSwapOption) String() string {
	return o.BuildHXSwapExpression()
}

func (o ModifiedHXSwapOption) BuildHXSwapExpression() string {
	ss := make([]string, len(o.Modifiers))
	for i, m := range o.Modifiers {
		ss[i] = " " + m.BuildModifierExpression()
	}

	return o.Option.String() + strings.Join(ss, "")
}

func (o ModifiedHXSwapOption) Modify(m HXSwapModifier) ModifiedHXSwapOption {
	o.Modifiers = append(o.Modifiers, m)
	return o
}

func (o ModifiedHXSwapOption) String() string {
	return o.BuildHXSwapExpression()
}

func (m UseViewTransitionAPI) BuildModifierExpression() string {
	return fmt.Sprintf("transition:%v", bool(m))
}

func (m SwapDelay) BuildModifierExpression() string {
	return fmt.Sprintf("swap:%s", time.Duration(m))
}

func (m SettleDelay) BuildModifierExpression() string {
	return fmt.Sprintf("settle:%s", time.Duration(m))
}

func (m IgnoreTitle) BuildModifierExpression() string {
	return fmt.Sprintf("ignoreTitle:%v", bool(m))
}

func (m ScrollTo) BuildModifierExpression() string {
	var selector string
	if m.Selector != "" {
		selector = ":" + m.Selector
	}

	return fmt.Sprintf("scroll%s:%s", selector, m.Side)
}

func (m Show) BuildModifierExpression() string {
	var selector string
	if m.Selector != "" {
		selector = ":" + m.Selector
	}

	return fmt.Sprintf("show%s:%s", selector, m.Side)
}

func (m FocusScroll) BuildModifierExpression() string {
	return fmt.Sprintf("focus-scroll:%v", m)
}

func (o TopOrBottom) String() string {
	if o {
		return "top"
	} else {
		return "bottom"
	}
}
