package core

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// Hierarchy is the root of a UIAutomator XML dump.
type Hierarchy struct {
	XMLName  xml.Name `xml:"hierarchy"`
	Rotation int      `xml:"rotation,attr"`
	Nodes    []Node   `xml:"node"`
}

// Node represents a single UI element in the hierarchy.
type Node struct {
	Index       int    `xml:"index,attr"`
	Text        string `xml:"text,attr"`
	ResourceID  string `xml:"resource-id,attr"`
	Class       string `xml:"class,attr"`
	Package     string `xml:"package,attr"`
	ContentDesc string `xml:"content-desc,attr"`
	Checkable   bool   `xml:"checkable,attr"`
	Checked     bool   `xml:"checked,attr"`
	Clickable   bool   `xml:"clickable,attr"`
	Enabled     bool   `xml:"enabled,attr"`
	Focusable   bool   `xml:"focusable,attr"`
	Focused     bool   `xml:"focused,attr"`
	Scrollable  bool   `xml:"scrollable,attr"`
	Selected    bool   `xml:"selected,attr"`
	Password    bool   `xml:"password,attr"`
	BoundsStr   string `xml:"bounds,attr"`
	Hint        string `xml:"hint,attr"`
	Children    []Node `xml:"node"`
}

// Rect represents a rectangle on screen.
type Rect struct {
	Left   int
	Top    int
	Right  int
	Bottom int
}

// Point represents a screen coordinate.
type Point struct {
	X int
	Y int
}

// Element is a flattened, query-friendly representation of a UI node.
type Element struct {
	Text        string
	ResourceID  string
	ContentDesc string
	Class       string
	Package     string
	Bounds      Rect
	Clickable   bool
	Enabled     bool
	Scrollable  bool
	Selected    bool
	Focused     bool
	Checked     bool
	Hint        string
	Children    []*Element
	Parent      *Element
}

// Center returns the center point of the element.
func (e *Element) Center() Point {
	return Point{
		X: (e.Bounds.Left + e.Bounds.Right) / 2,
		Y: (e.Bounds.Top + e.Bounds.Bottom) / 2,
	}
}

// Width returns the element width.
func (e *Element) Width() int {
	return e.Bounds.Right - e.Bounds.Left
}

// Height returns the element height.
func (e *Element) Height() int {
	return e.Bounds.Bottom - e.Bounds.Top
}

// Tap taps the center of this element.
func (e *Element) Tap(ctx context.Context, dev Device) error {
	c := e.Center()
	return dev.Tap(ctx, c.X, c.Y)
}

// Predicate is a function that filters elements.
type Predicate func(*Element) bool

// ParseDump parses UIAutomator XML into a Hierarchy.
func ParseDump(xmlData []byte) (*Hierarchy, error) {
	var h Hierarchy
	if err := xml.Unmarshal(xmlData, &h); err != nil {
		return nil, fmt.Errorf("parsing UI dump: %w", err)
	}
	return &h, nil
}

// parseBounds parses a bounds string like "[0,0][1080,2400]" into a Rect.
func parseBounds(s string) Rect {
	// Format: [left,top][right,bottom]
	s = strings.ReplaceAll(s, "][", ",")
	s = strings.Trim(s, "[]")
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return Rect{}
	}
	left, _ := strconv.Atoi(parts[0])
	top, _ := strconv.Atoi(parts[1])
	right, _ := strconv.Atoi(parts[2])
	bottom, _ := strconv.Atoi(parts[3])
	return Rect{Left: left, Top: top, Right: right, Bottom: bottom}
}

// nodeToElement converts a Node tree into a flat Element tree.
func nodeToElement(n *Node, parent *Element) *Element {
	e := &Element{
		Text:        n.Text,
		ResourceID:  n.ResourceID,
		ContentDesc: n.ContentDesc,
		Class:       n.Class,
		Package:     n.Package,
		Bounds:      parseBounds(n.BoundsStr),
		Clickable:   n.Clickable,
		Enabled:     n.Enabled,
		Scrollable:  n.Scrollable,
		Selected:    n.Selected,
		Focused:     n.Focused,
		Checked:     n.Checked,
		Hint:        n.Hint,
		Parent:      parent,
	}
	for i := range n.Children {
		child := nodeToElement(&n.Children[i], e)
		e.Children = append(e.Children, child)
	}
	return e
}

// ElementFinder queries elements from a parsed UI dump.
type ElementFinder struct {
	roots []*Element
}

// NewElementFinder creates an ElementFinder from a parsed hierarchy.
func NewElementFinder(h *Hierarchy) *ElementFinder {
	var roots []*Element
	for i := range h.Nodes {
		roots = append(roots, nodeToElement(&h.Nodes[i], nil))
	}
	return &ElementFinder{roots: roots}
}

// NewElementFinderFromXML parses XML and creates an ElementFinder.
func NewElementFinderFromXML(xmlData []byte) (*ElementFinder, error) {
	h, err := ParseDump(xmlData)
	if err != nil {
		return nil, err
	}
	return NewElementFinder(h), nil
}

// All returns all elements matching every given predicate.
func (f *ElementFinder) All(predicates ...Predicate) []*Element {
	var results []*Element
	for _, root := range f.roots {
		collectMatching(root, predicates, &results)
	}
	return results
}

// First returns the first element matching all predicates, or nil.
func (f *ElementFinder) First(predicates ...Predicate) *Element {
	for _, root := range f.roots {
		if e := findFirst(root, predicates); e != nil {
			return e
		}
	}
	return nil
}

// ByID finds the first element with the given resource ID.
func (f *ElementFinder) ByID(resourceID string) *Element {
	return f.First(func(e *Element) bool {
		return e.ResourceID == resourceID
	})
}

// ByText finds the first element with matching text.
// If exact is false, it does a case-insensitive contains match.
func (f *ElementFinder) ByText(text string, exact bool) *Element {
	return f.First(func(e *Element) bool {
		if exact {
			return e.Text == text
		}
		return strings.Contains(strings.ToLower(e.Text), strings.ToLower(text))
	})
}

// ByContentDesc finds the first element with matching content description.
func (f *ElementFinder) ByContentDesc(desc string) *Element {
	return f.First(func(e *Element) bool {
		return e.ContentDesc == desc
	})
}

// ByClass returns all elements with the given class name.
func (f *ElementFinder) ByClass(className string) []*Element {
	return f.All(func(e *Element) bool {
		return e.Class == className
	})
}

// HasText returns a predicate that matches elements containing the given text.
func HasText(text string) Predicate {
	lower := strings.ToLower(text)
	return func(e *Element) bool {
		return strings.Contains(strings.ToLower(e.Text), lower)
	}
}

// HasID returns a predicate that matches elements with the given resource ID.
func HasID(id string) Predicate {
	return func(e *Element) bool {
		return e.ResourceID == id
	}
}

// HasContentDesc returns a predicate that matches elements with the given content description.
func HasContentDesc(desc string) Predicate {
	return func(e *Element) bool {
		return strings.Contains(e.ContentDesc, desc)
	}
}

// IsClickable returns a predicate that matches clickable elements.
func IsClickable() Predicate {
	return func(e *Element) bool {
		return e.Clickable
	}
}

// IsScrollable returns a predicate that matches scrollable elements.
func IsScrollable() Predicate {
	return func(e *Element) bool {
		return e.Scrollable
	}
}

func collectMatching(e *Element, predicates []Predicate, results *[]*Element) {
	if matchesAll(e, predicates) {
		*results = append(*results, e)
	}
	for _, child := range e.Children {
		collectMatching(child, predicates, results)
	}
}

func findFirst(e *Element, predicates []Predicate) *Element {
	if matchesAll(e, predicates) {
		return e
	}
	for _, child := range e.Children {
		if found := findFirst(child, predicates); found != nil {
			return found
		}
	}
	return nil
}

func matchesAll(e *Element, predicates []Predicate) bool {
	for _, p := range predicates {
		if !p(e) {
			return false
		}
	}
	return true
}
