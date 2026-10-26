package sdin

import (
	"fmt"
	"strings"

	"github.com/ypsu/sdin/tokens"
)

// Parse parses sdin text into a Node.
// kind determines whether the input sdin is a struct, a list of nameless values, or form arguments (both named and nameless values).
// It must be [TFlatForm()], [TFlatList()], or [TFlatStruct()].
func Parse(kind Token, p string) (*Node, error) {
	n := &Node{Value: kind}

	ts, err := tokens.Tokenize(p)
	if err != nil {
		return nil, fmt.Errorf("sdin.TokenizeForParse: %v", err)
	}

	// Extract the first comment block as a file level comment.
	for i := 0; i < len(ts) && Kind(p[ts[i].Start:ts[i].End]).Trivia(); i++ {
		if p[ts[i].Start] == '\n' || i == len(ts)-1 {
			for _, t := range ts[:i+1] {
				n.Pre = append(n.Pre, p[t.Start:t.End])
			}
			ts = ts[i+1:]
			break
		}
	}

	// Parse.
	ts, err = parseBlock(n, p, ts)
	if err != nil {
		return nil, err
	}
	for _, t := range ts {
		n.PreClose = append(n.PreClose, p[t.Start:t.End])
	}
	return n, nil
}

// parseBlock parses the tokens into node n's children up until the next closing token.
// The remaining tokens (including the closing token and its comments) are then returned.
func parseBlock(n *Node, p string, ts []tokens.TSlice) ([]tokens.TSlice, error) {
	var defineLine map[string]int
	var lastparts []string // e.g. ["a" "b" "c"] if the last key was "a.b.c"
	var lastnodes []*Node  // contains 2 elements if last key was "a.b.c", 3 if it was "a.b.c+"
	isStruct, isList, isMap := n.Kind().Struct(), n.Kind().List(), n.Kind() == "map("
	if isStruct {
		defineLine = map[string]int{}
	}

	// Struct token stream format: pretokens* nametoken valuetoken posttoken? (childtokens* closetokens+)?
	// List token stream format: pretokens* valuetoken posttoken? (childtokens* closetokens+)?
	// Form token stream format: pretokens* nametoken? valuetoken posttoken? (childtokens* closetokens+)?
	// Closetokens: pretokens* closetoken posttoken?
	// Each pretoken and posttoken must be either a newline or a comment (trivia tokens).
	// The nametokens must be bare tokens.
	// The valuetokens can be bare tokens, strings, or block opener tokens.
	// In structs there can be no newline or comment between the key and the value.
	//
	// Each iteration this loop processes one key+value (for structs) or value (for lists).
	// The processed tokens are then removed from ts.
	for len(ts) > 0 {
		// t returns the kth token.
		t := func(k int) string { return p[ts[k].Start:ts[k].End] }

		var (
			pte   int    // pretokens end
			named bool   // if this is a name-value pair
			np    int    // name pos
			nt    string // name token
			vp    int    // value pos
			vt    string // value token
			line  int    // value pos lineno
			ptp   int    // posttoken pos, equal to vp if there's nothing
			pn    *Node  // parent node into which the value should be inserted (different from n if key is a path)
			cn    *Node  // child node which will be inserted
		)
		for pte < len(ts) && Kind(t(pte)).Trivia() {
			pte++
		}
		if pte == len(ts) {
			// No more values found, exit.
			break
		}
		np, nt, vp, line = pte, t(pte), pte, ts[pte].Line

		if !n.Kind().Flat() && Kind(nt).OpenerByte() == n.Kind().ComplementByte() {
			// No more values found, exit successfully.
			return ts, nil
		}
		if Kind(nt).Closing() {
			if n.Kind().Flat() {
				return nil, fmt.Errorf("sdin.InvalidTopBlockClose line=%d", line)
			}
			return nil, fmt.Errorf("sdin.InvalidBlockClose line=%d startline=%d gotchar=%s wantchar=%c", line, n.Line, nt, n.Kind().ComplementByte())
		}
		named = Kind(nt).Name()
		nt = strings.TrimSuffix(nt, ":")
		if named && nt == "" {
			return nil, fmt.Errorf("sdin.EmptyName line=%d: name must be non-empty", line)
		}
		if isStruct && !named {
			return nil, fmt.Errorf("sdin.UnnamedStructMember line=%d value=%s: all struct members must be name-value pairs (or are you missing a colon?)", line, nt)
		}
		if isStruct && !Kind(strings.TrimSuffix(nt, "+")).IdentifierPath() {
			return nil, fmt.Errorf("sdin.InvalidStructFieldName line=%d name=%q: struct field name must be a dot separated identifier path", line, nt)
		}
		if isList && named {
			return nil, fmt.Errorf("sdin.NamedValueInList line=%d name=%q: list members cannot have names", line, nt)
		}
		if isMap && named {
			return nil, fmt.Errorf("sdin.NamedValueInMap line=%d name=%q: map() arguments cannot have names", line, nt)
		}
		if isMap && len(n.Children)%2 == 0 && !Kind(nt).Scalar() {
			return nil, fmt.Errorf("sdin.NonScalarMapKey line=%d: map() keys must be scalars", line)
		}
		if named {
			vp++
		}
		if vp == len(ts) {
			return nil, fmt.Errorf("sdin.EOFAfterName line=%d", line)
		}

		ptp, vt = vp, t(vp)
		if vp+1 < len(ts) && Kind(t(vp+1)).Trivia() {
			ptp++
		}
		if named && Kind(vt).Trivia() {
			return nil, fmt.Errorf("sdin.NewlineAfterName line=%d: key names and values must be on the same line", line)
		}
		if named && Kind(vt).Closing() {
			return nil, fmt.Errorf("sdin.MissingValue line=%d: found block closer instead of value", line)
		}
		if named && Kind(vt).Name() {
			return nil, fmt.Errorf("sdin.MissingValue line=%d name=%s: missing value for this field (and values cannot have colons)", line, nt)
		}
		if vt == "(" {
			// Forms would be error-prone without this rule.
			// `[Tag()]` and `[Tag ()]` are two valid lists otherwise; this could be surprising.
			return nil, fmt.Errorf("sdin.MissingFormTag line=%d: forms must have a tag name", line)
		}

		pn, cn = n, &Node{Value: Token{vt}, Line: line}
		for i := range np {
			cn.Pre = append(cn.Pre, t(i))
		}
		if ptp != vp {
			cn.Post = t(ptp)
		}
		ts = ts[ptp+1:]

		// Process the key name in struct.
		if isStruct && (strings.IndexByte(nt, '.') != -1 || strings.HasSuffix(nt, "+")) {
			// The key name is a path.
			// Split the identifier by dots in order to change into the right substruct.
			fullid, isFlatList := strings.CutSuffix(nt, "+")
			if strings.HasSuffix(fullid, ".") || strings.Contains(fullid, "..") {
				return nil, fmt.Errorf("sdin.UnexpectedDotsInName line=%d name=%q", line, fullid)
			}
			parts, prefixedparts := splitID(fullid)

			i := 0 // i contains the first difference
			for i < min(len(parts), len(lastnodes)) && parts[i] == lastparts[i] {
				i, pn = i+1, lastnodes[i]
			}
			isListContinuation := isFlatList && len(parts) == len(lastparts) && i == len(parts)

			// Check for redefinition and create the new child node.
			if isListContinuation {
				pn = lastnodes[len(lastnodes)-1]
			} else {
				if i > 0 && (i == len(parts) || i == len(lastparts)) {
					return nil, fmt.Errorf("sdin.RedefinedPrevField line=%d name=%s prevname=%s", line, fullid, strings.Join(lastparts, "."))
				}

				if prevline, ok := defineLine[prefixedparts[i]]; ok {
					return nil, fmt.Errorf("sdin.RedefinedFlatField line=%d name=%q prevline=%d", line, prefixedparts[i], prevline)
				}

				lastnodes = lastnodes[:i]
				for j := i; j < len(parts)-1; j++ {
					defineLine[prefixedparts[j]] = line
					tn := &Node{Name: parts[j], Value: TFlatStruct(), Line: line}
					pn.Children = append(pn.Children, tn)
					pn, lastnodes = tn, append(lastnodes, tn)
				}
				defineLine[prefixedparts[len(parts)-1]] = line
				lastparts = parts
				if isFlatList {
					tn := &Node{Name: parts[len(parts)-1], Value: TFlatList(), Line: line}
					pn.Children = append(pn.Children, tn)
					pn, lastnodes = tn, append(lastnodes, tn)
				}
			}
			if !isFlatList {
				cn.Name = parts[len(parts)-1]
			}
		} else if isStruct {
			if prevline, ok := defineLine[nt]; ok {
				return nil, fmt.Errorf("sdin.RedefinedField line=%d name=%q prevline=%d", line, nt, prevline)
			}
			defineLine[nt] = line
			lastparts, lastnodes = lastparts[:0], lastnodes[:0]
			cn.Name = nt
		} else if named {
			cn.Name = nt
		}
		pn.Children = append(pn.Children, cn)

		if Kind(vt).Scalar() {
			continue
		}

		var err error
		ts, err = parseBlock(cn, p, ts)
		if err != nil {
			return nil, err
		}
		v := 0 // everything up until the next close sign and the next newline belongs to closetokens
		for v < len(ts) && Kind(t(v)).Trivia() {
			cn.PreClose = append(cn.PreClose, t(v))
			v++
		}
		v++
		if v < len(ts) && Kind(t(v)).Trivia() {
			cn.PostClose = t(v)
			v++
		}
		ts = ts[v:]
	}

	if !n.Kind().Flat() {
		return nil, fmt.Errorf("sdin.UnclosedNode startline=%d", n.Line)
	}
	return ts, nil
}

// splitID splits an ID into its components and prefixes.
// "a.b.c" -> ["a" "b" "c"] ["a" "a.b" "a.b.c"]
func splitID(id string) (parts, prefixedparts []string) {
	parts = strings.Split(id, ".")
	prefixedparts = make([]string, len(parts))
	prefixedparts[0] = parts[0]
	for i := 1; i < len(parts); i++ {
		prefixedparts[i] = id[:len(prefixedparts[i-1])+1+len(parts[i])]
	}
	return parts, prefixedparts
}
