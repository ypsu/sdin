// Package sdin provides the structural, parsing, and formatting functions for sdin objects.
package sdin

import (
	"fmt"
	"strings"
)

// Token represents an sdin token.
type Token struct{ s string }

// Node represents an sdin value.
type Node struct {
	// List of comments and newline tokens that appear before the value.
	Pre []string

	// Name, Value, and Post tokens are all defined on a single line.
	// In structs, Name is an identifier which matches the regex `[a-zA-Z_][a-zA-Z0-9_]*`.
	// In forms, Name can be any bare token.
	Name  string // the bare token before the colon, or empty if this is an unnamed node (e.g. in lists)
	Value Token  // the value token; can be `[`, `{`, or `tagname(` for composite nodes (`:[`, `:{`, `:(` for the flat versions)
	Post  string // the newline or comment, if any, after the value (or opening delimiter in composite nodes)

	// The 1-based line on which the Value token was defined.
	Line int

	// Fields relevant only for struct/list/form nodes follow.
	Children []*Node

	// List of comments and newline tokens that appear before the matching }]) token for {[( values.
	PreClose []string

	// For {[( values this token appears after the matching }]) token.
	// Can be a comment for a sidecomment, a newline to indicate EOL, or empty to keep the object on a single line.
	PostClose string
}

// Kind returns the Kind object that can be used to query the properties of this node's value.
func (n *Node) Kind() Kind {
	return Kind(n.Value.s)
}

// Directives returns the sdin values from the #% comments.
// It transforms invalid lines into error() directives with the parse error.
// Returns nil if no directives were found.
func (n *Node) Directives() []*Node {
	count := 0
	for _, comment := range n.Pre {
		if strings.HasPrefix(comment, "#%") {
			count++
		}
	}
	if strings.HasPrefix(n.Post, "#%") {
		count++
	}
	if count == 0 {
		return nil
	}
	an := &Node{Value: TForm(":"), Children: make([]*Node, 0, count), Post: "\n", PostClose: "\n"}

	add := func(line int, comment string) {
		if strings.HasPrefix(comment, "#%") {
			if tn, err := Parse(TFlatForm(), comment[2:]); err == nil {
				if n.Line > 0 {
					tn.Line = line
					tn.Replace(func(ch *Node) (*Node, error) {
						ch.Line = line
						return ch, nil
					})
				}
				an.Children = append(an.Children, tn.Children...)
			} else {
				msg := &Node{Value: MakeToken(fmt.Sprintf("sdin.InvalidDirective line=%d: %v", line, err))}
				an.Children = append(an.Children, &Node{PostClose: "\n", Value: TForm("error"), Children: []*Node{msg}})
			}
		}
	}

	// max() is needed to handle the root node's preamble comments (the root node's line number is zero).
	startLine := max(n.Line-len(n.Pre), 1)
	for i, comment := range n.Pre {
		add(startLine+i, comment)
	}
	add(n.Line, n.Post)
	return an.Children
}

// FormFields returns a new struct Node whose children are the named values of `n`.
// Always returns an empty node for list or nil nodes, and an equivalent node for struct nodes.
// This is primarily intended for forms, which can contain both named and unnamed values.
// This method never returns nil.
// The children are copied by reference, not via deep copy, so modifying them modifies `n`'s children too.
func (n *Node) FormFields() *Node {
	m := NewNode("", TStruct())
	if n == nil {
		return m
	}
	m.Line = n.Line
	for _, ch := range n.Children {
		if ch.Name != "" {
			m.Children = append(m.Children, ch)
		}
	}
	return m
}

// FormValues returns a new list Node whose children are the unnamed values of `n`.
// Always returns an empty node for struct or nil nodes, and an equivalent node for list nodes.
// This is primarily intended for forms, which can contain both named and unnamed values.
// This method never returns nil.
// The children are copied by reference, not via deep copy, so modifying them modifies `n`'s children too.
func (n *Node) FormValues() *Node {
	m := NewNode("", TList())
	if n == nil {
		return m
	}
	m.Line = n.Line
	for _, ch := range n.Children {
		if ch.Name == "" {
			m.Children = append(m.Children, ch)
		}
	}
	return m
}

// Repr returns the byte representation of the Token as it would appear in an .sdin file.
func (t Token) Repr() string {
	return t.s
}

// MakeTokenFromRepr casts a raw token representation to Token.
// Only use it if you know what you are doing.
func MakeTokenFromRepr(s string) Token {
	return Token{s}
}
