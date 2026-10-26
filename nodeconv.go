package sdin

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// As converts a node into a T value.
// In structs, fields are fuzzily matched: case and underscores ignored.
// By default this function errors out when a struct doesn't have a matching field for a node.
// Create a dummy `_discardUnknownSDINFields struct{}` marker tag in the struct to discard non-matching fields.
// Implement [Interface.FromSDINNode] to override the default merging.
//
// Note that the returned value might retain references to `n` or its children.
// For example, `As[*sdin.Node]` will return `n` as is.
//
// If n is nil, As() returns an error.
func (n *Node) As[T any]() (T, error) {
	var t T
	if n == nil {
		return t, fmt.Errorf("sdin.AsFromNilNode")
	}
	if pt, ok := any(&t).(**Node); ok {
		*pt = n
		return t, nil
	}

	dst := reflect.ValueOf(&t)
	npbuf := make([]pathentry, 0, 8)
	err := n.into(dst.Elem(), nodepath{&npbuf, 0})
	return t, err
}

type pathentry struct {
	index int
	name  string
}

type nodepath struct {
	entries *[]pathentry
	len     int
}

func (np nodepath) add(index int, name string) nodepath {
	*np.entries = append((*np.entries)[:np.len], pathentry{index, name})
	return nodepath{np.entries, np.len + 1}
}

func (np nodepath) String() string {
	w := strings.Builder{}
	for _, e := range (*np.entries)[:np.len] {
		if w.Len() != 0 {
			w.WriteByte('.')
		}
		w.WriteString(e.name)
		if e.index != -1 {
			w.WriteString(strconv.Itoa(e.index))
		}
	}
	if w.Len() == 0 {
		return ""
	}
	return " path=" + w.String()
}

func (n *Node) into(dst reflect.Value, path nodepath) error {
	if _, ok := dst.Interface().(Node); ok {
		*dst.Addr().Interface().(*Node) = *n
		return nil
	}

	fromNode, isFromNode := dst.Addr().Interface().(fromNode)
	switch {
	case dst.Kind() == reflect.Pointer:
		if dst.IsZero() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		return n.into(dst.Elem(), path)

	case isFromNode:
		if err := fromNode.FromSDINNode(n); err != nil {
			return fmt.Errorf("sdin.FromSDINNode line=%d%s: %v", n.Line, path, err)
		}
		return nil

	case n.Kind().Scalar():
		if err := into(dst.Addr().Interface(), n.Value.s); err != nil {
			return fmt.Errorf("sdin.TokenInto line=%d%s: %v", n.Line, path, err)
		}
		return nil

	case dst.Kind() == reflect.Struct:
		if !n.Kind().Struct() {
			return fmt.Errorf("sdin.NonSDINStructIntoGoStruct line=%d%s got=%q: node must be an sdin struct in order to convert it to a Go struct", n.Line, path, n.Value.s)
		}
		if err := n.intoStruct(dst, path); err != nil {
			return err
		}
		return nil

	case dst.Kind() == reflect.Map:
		if !n.Kind().Struct() && n.Value.s != "map(" {
			return fmt.Errorf("sdin.NonSDINStructIntoMap line=%d%s got=%q: node must be an sdin struct or map() in order to convert it to a map", n.Line, path, n.Value.s)
		}
		if err := n.intoMap(dst, path); err != nil {
			return err
		}
		return nil

	case dst.Kind() == reflect.Slice:
		if !n.Kind().List() {
			return fmt.Errorf("sdin.NonListIntoSlice line=%d%s got=%q: node must be a list to convert it to a slice", n.Line, path, n.Value.s)
		}
		dst.Set(reflect.MakeSlice(dst.Type(), len(n.Children), len(n.Children)))
		for i, ch := range n.Children {
			if err := ch.into(dst.Index(i), path.add(i, "")); err != nil {
				return err
			}
		}
		return nil
	}

	if n.Kind().Form() {
		return fmt.Errorf("sdin.UnresolvedForm line=%d%s name=%q", n.Line, path, n.Value.s)
	}
	return fmt.Errorf("sdin.UnsupportedIntoKind line=%d%s kind=%s: check sdin.Interface for adding support", n.Line, path, dst.Kind())
}

func (n *Node) intoStruct(dst reflect.Value, path nodepath) error {
	discard := false
	typ, fields := dst.Type(), make(map[string]reflect.Value, dst.NumField())
	for i := range typ.NumField() {
		f, name := dst.Field(i), typ.Field(i).Name
		if name == "_discardUnknownSDINFields" {
			discard = true
		}
		if !f.CanSet() {
			continue
		}
		name = strings.ReplaceAll(strings.ToLower(name), "_", "")
		fields[name] = f
	}

	for _, ch := range n.Children {
		name := strings.ReplaceAll(strings.ToLower(ch.Name), "_", "")
		f, ok := fields[name]
		if !ok && !discard {
			return fmt.Errorf("sdin.StructFieldNotFound line=%d%s structtype=%T (add a _discardUnknownSDINFields marker tag to the struct to ignore this)", ch.Line, path.add(-1, ch.Name), dst.Interface())
		} else if !ok {
			continue
		}
		if err := ch.into(f, path.add(-1, ch.Name)); err != nil {
			return err
		}
	}
	return nil
}

func (n *Node) intoMap(dst reflect.Value, path nodepath) error {
	if dst.IsNil() {
		dst.Set(reflect.MakeMap(dst.Type()))
	}
	keyType, elemType := dst.Type().Key(), dst.Type().Elem()
	if n.Kind().Struct() {
		for _, ch := range n.Children {
			k, v := reflect.New(keyType).Elem(), reflect.New(elemType).Elem()
			if ft, ok := k.Addr().Interface().(fromToken); ok {
				if err := ft.FromSDINToken(Token{ch.Name}); err != nil {
					return fmt.Errorf("sdin.FromMapKey line=%d%s: %v", ch.Line, path.add(-1, ch.Name), err)
				}
			} else if keyType.Kind() == reflect.String {
				k.SetString(ch.Name)
			} else if err := into(k.Addr().Interface(), ch.Name); err != nil {
				return fmt.Errorf("sdin.ParseMapKey line=%d%s: %v", ch.Line, path.add(-1, ch.Name), err)
			}
			if err := ch.into(v, path.add(-1, ch.Name)); err != nil {
				return err
			}
			dst.SetMapIndex(k, v)
		}
		return nil
	}

	// n.Value == "map(": the values are pairs.
	if len(n.Children)%2 != 0 {
		return fmt.Errorf("sdin.OddMapValues line=%d lastkeyline=%d (map() needs exactly 2n values)", n.Line, n.Children[len(n.Children)-1].Line)
	}
	for i := 0; i < len(n.Children); i += 2 {
		k, v := reflect.New(keyType).Elem(), reflect.New(elemType).Elem()
		if err := n.Children[i].into(k, path.add(-1, n.Children[i].Value.s)); err != nil {
			return err
		}
		if err := n.Children[i+1].into(v, path.add(-1, n.Children[i].Value.s)); err != nil {
			return err
		}
		dst.SetMapIndex(k, v)
	}
	return nil
}

// NewNode creates an sdin node.
// Uses [Interface.IntoSDINNode] or [Interface.IntoSDINToken] if implemented.
// Pass "" as name if this is meant to be a list value where the name must be empty.
// Panics for unsupported primitive types.
//
// Use NewNode(n.Name, n) to make a deep clone of an existing node.
func NewNode(name string, value any) *Node {
	n := &Node{Name: name, Post: "\n"}
	if value == nil {
		return n
	}
	n.from(reflect.ValueOf(value))
	if n.PostClose == "" && n.Kind().Opening() {
		if _, isNode := value.(*Node); !isNode {
			// Only if value is not a *Node; the copies of those should remain exact.
			n.PostClose = "\n"
		}
	}
	return n
}

// NewFlatNode creates an sdin node.
// Uses [Interface.IntoSDINNode] or [Interface.IntoSDINToken] if implemented.
// Pass "" as name if this is meant to be a list value where the name must be empty.
// If the result is a struct, a list, or an empty form, then the node is marked as flat.
// Panics for unsupported primitive types.
func NewFlatNode(name string, value any) *Node {
	n := NewNode(name, value)
	switch n.Value.s {
	case "{":
		n.Value.s = ":{"
	case "[":
		n.Value.s = ":["
	case "(":
		n.Value.s = ":("
	}
	return n
}

// from merges the node tree created from the passed in argument into the current node.
func (n *Node) from(rv reflect.Value) {
	v := rv.Interface()
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		return
	}
	t := rv.Type()

	if Tokenizable(v) {
		n.Value = MakeToken(v)
		return
	}
	if reflect.PointerTo(t).Implements(reflect.TypeFor[intoToken]()) {
		if !rv.CanAddr() {
			panic(fmt.Sprintf("sdin.PointerWouldImplementIntoSDINToken type=%T: pass a pointer to NewNode", v))
		}
		n.Value = rv.Addr().Interface().(intoToken).IntoSDINToken()
		return
	}

	if nn, ok := v.(Node); ok {
		name := n.Name
		*n = *nn.clone()
		n.Name = name
		return
	}
	if nn, ok := v.(*Node); ok {
		name := n.Name
		*n = *nn.clone()
		n.Name = name
		return
	}
	if in, ok := v.(intoNode); ok {
		name := n.Name
		*n = *in.IntoSDINNode()
		n.Name = name
		return
	}
	if reflect.PointerTo(t).Implements(reflect.TypeFor[intoNode]()) {
		if !rv.CanAddr() {
			panic(fmt.Sprintf("sdin.PointerWouldImplementIntoSDINNode type=%T: pass a pointer to NewNode", v))
		}
		name := n.Name
		*n = *rv.Addr().Interface().(intoNode).IntoSDINNode()
		n.Name = name
		return
	}

	if rv.Kind() == reflect.Slice {
		if !n.Kind().List() {
			pre := n.Pre
			*n = *NewNode(n.Name, MakeTokenFromRepr("["))
			n.Pre = pre
		}
		for i := range rv.Len() {
			nn := &Node{Post: "\n"}
			nn.from(rv.Index(i))
			n.Children = append(n.Children, nn)
		}
		if len(n.Children) == 0 {
			n.Post = ""
		}
		return
	}

	if rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		n.from(rv.Elem())
		return
	}

	if rv.Kind() == reflect.Struct {
		if !n.Kind().Struct() {
			pre := n.Pre
			*n = *NewNode(n.Name, MakeTokenFromRepr("{"))
			n.Pre = pre
		}
		n.fromStruct(rv)
		if len(n.Children) == 0 {
			n.Post = ""
		}
		return
	}

	if rv.Kind() == reflect.Map {
		if n.Value.s != "map(" {
			pre := n.Pre
			*n = *NewNode(n.Name, MakeTokenFromRepr("map("))
			n.Pre, n.PostClose = pre, "\n"
		}
		n.fromMap(rv)
		if len(n.Children) == 0 {
			n.Post = ""
		}
		return
	}

	panic(fmt.Sprintf("sdin.UnsupportedNodeFromType type=%T kind=%s: check sdin.Interface for adding support", v, rv.Kind()))
}

func (n *Node) fromStruct(rv reflect.Value) {
	nodeFields := make(map[string]*Node, len(n.Children))
	for _, ch := range n.Children {
		nodeFields[ch.Name] = ch
	}

	typ := rv.Type()
	for i := range typ.NumField() {
		if !typ.Field(i).IsExported() {
			continue
		}
		fv := rv.Field(i)
		if fv.Kind() == reflect.Pointer && fv.IsNil() {
			continue
		}

		name := typ.Field(i).Name
		nn, ok := nodeFields[name]
		if !ok && fv.IsZero() {
			// Ignore zero fields.
			// Set the field to zero if the original node had a child for it though.
			continue
		}
		if !ok {
			nn = NewNode(name, Identifier("nil"))
			nodeFields[name] = nn
			n.Children = append(n.Children, nn)
		}
		nn.from(fv)
	}
}

func (n *Node) fromMap(rv reflect.Value) {
	nodeFields := make(map[string]*Node, len(n.Children))
	for i := 0; i+1 < len(n.Children); i += 2 {
		nodeFields[n.Children[i].Value.s] = n.Children[i+1]
	}

	keys := rv.MapKeys()
	slices.SortFunc(keys, func(a, b reflect.Value) int {
		ta, tb := tokenFrom(a.Interface()), tokenFrom(b.Interface())
		return cmp.Compare(ta, tb)
	})
	for _, k := range keys {
		v := rv.MapIndex(k)
		if v.Kind() == reflect.Pointer && v.IsNil() {
			continue
		}

		name := tokenFrom(k.Interface())
		nv, ok := nodeFields[name]
		if !ok {
			nk := &Node{}
			nv = &Node{Post: "\n"}
			nodeFields[name] = nv
			nk.from(k)
			n.Children = append(n.Children, nk, nv)
		}
		nv.from(v)
	}
}

// clone makes a deep clone of the node, its slices, and its children.
func (n *Node) clone() *Node {
	nn := *n
	nn.Pre = slices.Clone(n.Pre)
	nn.PreClose = slices.Clone(n.PreClose)
	nn.Children = slices.Clone(n.Children)
	for i, ch := range nn.Children {
		nn.Children[i] = ch.clone()
	}
	return &nn
}

// Unmarshal parses s into a *Node and then converts that into a value of the given type.
// See [Node.As] for the details.
func Unmarshal[T any](s string) (T, error) {
	var v T
	rv := reflect.ValueOf(&v)
	var n *Node
	var err error
	if rv.Elem().Kind() == reflect.Slice {
		n, err = Parse(TFlatList(), s)
	} else {
		n, err = Parse(TFlatStruct(), s)
	}
	if err != nil {
		return v, fmt.Errorf("sdin.ParseForUnmarshal: %v", err)
	}
	npbuf := make([]pathentry, 0, 8)
	err = n.into(rv.Elem(), nodepath{&npbuf, 0})
	if err != nil {
		return v, fmt.Errorf("sdin.IntoForUnmarshal: %v", err)
	}
	return v, nil
}

// Marshal converts v to a *Node and formats it into a string.
// Formats into a sdin list if v is a slice.
// Panics if v contains an unsupported primitive type.
func Marshal(v any) string {
	n := NewNode("", v)
	if n.Kind().Struct() {
		n.Value = TFlatStruct()
	} else if n.Kind().List() {
		n.Value = TFlatList()
	}
	return n.Format()
}
