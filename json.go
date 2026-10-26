package sdin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode"

	"github.com/ypsu/sdin/tokens"
)

// MarshalJSON renders node as a JSON value.
// This implements json.Marshaler.
func (n *Node) MarshalJSON() ([]byte, error) {
	w := &bytes.Buffer{}
	if err := n.marshalJSON(w, 0); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func (n *Node) marshalJSON(w *bytes.Buffer, depth int) error {
	switch {
	case n.Kind().List() && len(n.Children) == 0:
		w.WriteString("[]")

	case n.Kind().Struct() && len(n.Children) == 0:
		w.WriteString("{}")

	case n.Value.s == "map(" && len(n.Children) == 0:
		w.WriteString("{}")

	case n.Kind().List():
		w.WriteString("[\n")
		for i, ch := range n.Children {
			writeTabs(w, depth+1)
			if err := ch.marshalJSON(w, depth+1); err != nil {
				return err
			}
			if i+1 != len(n.Children) {
				w.WriteByte(',')
			}
			w.WriteByte('\n')
		}
		writeTabs(w, depth)
		w.WriteByte(']')

	case n.Kind().Struct():
		w.WriteString("{\n")
		for i, ch := range n.Children {
			writeTabs(w, depth+1)
			fmt.Fprintf(w, "%q", ch.Name)
			w.WriteString(": ")
			if err := ch.marshalJSON(w, depth+1); err != nil {
				return err
			}
			if i+1 != len(n.Children) {
				w.WriteByte(',')
			}
			w.WriteByte('\n')
		}
		writeTabs(w, depth)
		w.WriteByte('}')

	case n.Value.s == "map(":
		w.WriteString("{\n")
		if len(n.Children)%2 != 0 {
			lch := n.Children[len(n.Children)-1]
			return fmt.Errorf("sdin.MissingMapValue line=%d field=%s", lch.Line, lch.Value.Repr())
		}
		for i := 0; i+1 < len(n.Children); i += 2 {
			writeTabs(w, depth+1)
			key := n.Children[i].Value.s
			if s, err := n.Children[i].As[string](); err == nil {
				key = s
			}
			js, _ := json.Marshal(key)
			fmt.Fprintf(w, "%s: ", js)
			if err := n.Children[i+1].marshalJSON(w, depth+1); err != nil {
				return err
			}
			if i+3 < len(n.Children) {
				w.WriteByte(',')
			}
			w.WriteByte('\n')
		}
		writeTabs(w, depth)
		w.WriteByte('}')

	default:
		v, err := n.Value.As[tokens.JSONValue]()
		if err != nil {
			return fmt.Errorf("sdin.ParseValue line=%d: %v", n.Line, err)
		}
		w.WriteString(string(v))
	}
	return nil
}

func writeTabs(w *bytes.Buffer, depth int) {
	for range depth {
		w.WriteByte('\t')
	}
}

// UnmarshalJSON parses JSON data into this Node.
// This implements json.Unmarshaler.
func (n *Node) UnmarshalJSON(p []byte) error {
	n.Post, n.Children, n.PreClose, n.PostClose = "\n", nil, nil, ""
	buf := bytes.NewBuffer(p)
	decoder := json.NewDecoder(buf)
	decoder.UseNumber()
	if err := n.unmarshalJSON(decoder); err != nil {
		line := bytes.Count(p[:decoder.InputOffset()], []byte("\n")) + 1
		return fmt.Errorf("sdin.DecodeJSON line=%d: %v", line, err)
	}
	if decoder.More() {
		return fmt.Errorf("sdin.PartialJSONDecode: input had more than one value")
	}
	return nil
}

func (n *Node) unmarshalJSON(decoder *json.Decoder) error {
	t, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("sdin.GetNextJSONToken: %v", err)
	}
	n.Children = nil
	switch v := t.(type) {
	case json.Delim:
		switch v {
		case '[':
			n.Value.s = "["
			for decoder.More() {
				nn := &Node{Post: "\n"}
				if err := nn.unmarshalJSON(decoder); err != nil {
					return fmt.Errorf("sdin.UnmarshalJSONList index=%d: %v", len(n.Children), err)
				}
				n.Children = append(n.Children, nn)
			}
			if _, err := decoder.Token(); err != nil {
				return fmt.Errorf("sdin.ReadJSONListCloseToken: %v", err)
			}
		case '{':
			n.Value.s = "{"
			for decoder.More() {
				nn := &Node{Post: "\n"}
				kt, err := decoder.Token()
				if err != nil {
					return fmt.Errorf("sdin.ReadJSONObjectKey: %v", err)
				}
				k, ok := kt.(string)
				if !ok {
					return fmt.Errorf("sdin.NonStringJSONObjectKey token=%q", kt)
				}
				nn.Name = k
				if err := nn.unmarshalJSON(decoder); err != nil {
					return fmt.Errorf("sdin.UnmarshalJSONObject key=%q: %v", k, err)
				}
				n.Children = append(n.Children, nn)
			}
			if _, err := decoder.Token(); err != nil {
				return fmt.Errorf("sdin.ReadJSONObjectCloseToken: %v", err)
			}

			needsMap := false
		keycheck:
			for _, ch := range n.Children {
				for i, r := range ch.Name {
					if i == 0 && unicode.IsDigit(r) || r != '_' && !unicode.IsDigit(r) && !unicode.IsLetter(r) {
						needsMap = true
						break keycheck
					}
				}
			}
			if !needsMap {
				break
			}
			n.Value.s = "map("
			var mapChildren []*Node
			for _, ch := range n.Children {
				mapChildren = append(mapChildren, &Node{Value: MakeTokenFromRepr(fmt.Sprintf("%q", ch.Name))}, &Node{Value: ch.Value, Post: "\n", Children: ch.Children})
			}
			n.Children = mapChildren
		default:
			return fmt.Errorf("sdin.UnsupportedJSONDelim type=%T token=%v", v, t)
		}
	case bool:
		n.Value.s = fmt.Sprintf("%t", v)
	case string:
		n.Value.s = fmt.Sprintf("%q", v)
	case json.Number:
		n.Value.s = v.String()
	case nil:
		n.Value.s = "nil"
	default:
		return fmt.Errorf("sdin.UnsupportedJSONToken type=%T token=%v", v, t)
	}
	return nil
}
