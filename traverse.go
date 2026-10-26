package sdin

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Get traverses into a child node specified via `path`.
// Returns nil if no such child exists or `n` itself is nil.
// `path` is a dot-separated list of integers and identifiers.
// For integers, Get traverses to the child with that index.
// For identifiers each traversal involves a linear scan of all child nodes.
// Create custom child maps if faster traversal is desired.
//
// If `n` is `{a:{} b:{bb+:"x" bb+:"y"} c:{}}`, then `Get("b.bb.1").Value` will be `"y"`.
func (n *Node) Get(path string) *Node {
pathLoop:
	for component := range strings.SplitSeq(path, ".") {
		if n == nil || component == "" {
			return nil
		}
		if i, err := strconv.Atoi(component); err == nil { // on success
			if i < 0 || i >= len(n.Children) {
				return nil
			}
			n = n.Children[i]
			continue
		}
		for _, ch := range n.Children {
			if ch.Name == component {
				n = ch
				continue pathLoop
			}
		}
		return nil
	}
	return n
}

// Replace recursively traverses all children, calls the passed function for them, and replaces them in the parent node with the returned node.
// Return `n, nil` to keep a node intact.
// If the function returns nil, the node is deleted from the parent's children.
// Can be used to implement function calls or to simply visit all nodes.
// The line number and node name are copied from the original child when the replacement node has those fields unset.
//
// Note that the replacer function isn't called for the passed in root node because Replace() doesn't know its parent.
// If fn() returns a non-nil error, the loop aborts and the tree remains partially processed.
func (n *Node) Replace(fn func(n *Node) (*Node, error)) error {
	defer func() { n.Children = slices.DeleteFunc(n.Children, func(n *Node) bool { return n == nil }) }()

	for i, ch := range n.Children {
		if err := ch.Replace(fn); err != nil {
			return err
		}
		name, line := ch.Name, ch.Line
		newch, err := fn(ch)
		if err != nil {
			return fmt.Errorf("sdin.Replace line=%d name=%s: %v", line, name, err)
		}
		if newch != nil && newch.Line == 0 {
			newch.Line = line
		}
		if newch != nil && newch.Name == "" {
			newch.Name = name
		}
		n.Children[i] = newch
	}
	return nil
}
