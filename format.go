package sdin

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Format formats the given block or list node into well-indented text.
// Note that this might modify the node tree by adding/removing newlines where appropriate.
func (n *Node) Format() string {
	w, singleline := &strings.Builder{}, map[*Node]bool{}
	n.markSingleLineNodes(singleline)
	// markSingleLineNodes only checks the children's comments, so check the root's own here.
	if len(n.Pre) > 0 || len(n.PreClose) > 0 {
		delete(singleline, n)
	}
	if !singleline[n] {
		n.fixNewlines(singleline)
	}
	ts := n.tokenizeForFormat(nil, make([]byte, 0, 1024), 0, singleline)
	if len(ts) == 0 {
		return ""
	}

	// Add the missing EOF newline if the root node is multiline.
	// Single-line sdins can remain single line.
	if !strings.HasSuffix(ts[len(ts)-1], "\n") && !singleline[n] {
		ts = append(ts, "\n")
	}

	// Convert CRLF line endings to LF.
	for i, t := range ts {
		if Kind(t).Trivia() {
			ts[i] = strings.ReplaceAll(t, "\r\n", "\n")
		}
	}

	// Split the list of tokens into line blocks.
	// A line is a slice of tokens ending with a comment or newline.
	type lineblock struct {
		mintokens, maxtokens int // the non-comment token count range over the lines in this block
		indent               int
		lines                [][]string
	}
	var blocks []lineblock
	depth, continuable := 0, true
	for len(ts) > 0 {
		line := ts[:1]
		for len(line) < len(ts) && !Kind(line[len(line)-1]).Trivia() {
			line = ts[:len(line)+1]
		}
		ts = ts[len(line):]

		startDepth, endDepth, indent, canContinue, tokens, newrun := depth, depth, 0, continuable, 0, false
		continuable = true
		for _, t := range line {
			if t == "\t" {
				indent++
			} else if tokens == 0 && Kind(t).Opening() {
				tokens = 3
			} else if t[0] != '#' && t[0] != '\n' && depth == startDepth {
				tokens++
			}
			if strings.HasPrefix(t, "'") && strings.IndexByte(t, '\n') != -1 {
				// Multiline strings must be a standalone block.
				canContinue, continuable = false, false
			}
			if Kind(t).Closing() {
				endDepth, depth = endDepth-1, depth-1
				if depth < startDepth {
					newrun, continuable = true, false
				}
			} else if Kind(t).Opening() {
				endDepth, depth = endDepth+1, depth+1
			}
		}

		lb := lineblock{}
		if len(blocks) > 0 {
			lb = blocks[len(blocks)-1]
		}
		newrun = newrun || !canContinue
		newrun = newrun || tokens == 0
		newrun = newrun || len(blocks) == 0
		newrun = newrun || endDepth != startDepth
		newrun = newrun || lb.mintokens == 0
		newrun = newrun || lb.indent != indent
		if newrun {
			blocks = append(blocks, lineblock{mintokens: tokens, maxtokens: tokens, indent: indent, lines: [][]string{line}})
		} else {
			blocks[len(blocks)-1].lines = append(lb.lines, line)
			blocks[len(blocks)-1].mintokens = min(lb.mintokens, tokens)
			blocks[len(blocks)-1].maxtokens = max(lb.maxtokens, tokens)
		}
	}

	needspace := func(aligned bool, idx int, lasttoken, curtoken string) bool {
		if lasttoken == "\t" || strings.HasSuffix(lasttoken, "\n") || curtoken == "\n" {
			return false
		}
		if (!aligned || idx >= 2) && lasttoken[len(lasttoken)-1] == ':' {
			return false
		}
		return !(Kind(lasttoken).Opening() && !strings.HasPrefix(curtoken, "#") || Kind(curtoken).Closing())
	}

	// Format each block independently.
	for _, block := range blocks {
		aligned := block.mintokens == 2 && block.maxtokens == 2
		// Compute the extra spaces needed for vertical alignment.
		firstfieldlen, totallen := 0, 0
		for _, line := range block.lines {
			firstfieldlen = max(firstfieldlen, utf8.RuneCountInString(line[block.indent]))
		}
		for _, line := range block.lines {
			ll, ntti := 0, -1 // line length, non-trivia token index
			for i, t := range line {
				if t != "\t" && t != " " {
					ntti++
				}
				if Kind(t).Trivia() {
					continue
				}
				if i > 0 && needspace(aligned, ntti, line[i-1], t) {
					ll++
				}
				if aligned && i == block.indent {
					ll += firstfieldlen
				} else {
					ll += utf8.RuneCountInString(t)
				}
			}
			totallen = max(totallen, ll)
		}
		// Emit the tokens and the extra alignment spaces.
		for _, line := range block.lines {
			ll, ntti := totallen, 0 // line length, non-trivia token index
			first := true
			for i, t := range line {
				// Align the start column of the comments.
				if !first && strings.HasPrefix(t, "#") {
					for range ll + 1 {
						w.WriteByte(' ')
					}
				}
				if !first && needspace(aligned, ntti, line[i-1], t) {
					w.WriteByte(' ')
					ll--
				}
				w.WriteString(reindentMultilineString(Token{t}, block.indent+1).s)
				ll -= utf8.RuneCountInString(t)
				// Align the starting column of the values in objects.
				if aligned && i == block.indent {
					ll -= firstfieldlen - utf8.RuneCountInString(t)
					for range firstfieldlen - utf8.RuneCountInString(t) {
						w.WriteByte(' ')
					}
				}
				if t != "\t" && t != " " {
					first = false
					ntti++
				}
			}
		}
	}

	return w.String()
}

func (n *Node) markSingleLineNodes(marks map[*Node]bool) {
	singleline := true
	if n.Kind().Opening() && n.Post != "" {
		// The closing sign must be on a new line.
		singleline = false
	}
	for _, ch := range n.Children {
		ch.markSingleLineNodes(marks)
		singleline = singleline && marks[ch] && len(ch.Pre) == 0 && len(ch.PreClose) == 0 && ch.Post == "" && ch.PostClose == ""
	}
	if singleline {
		marks[n] = true
	}
}

func (n *Node) fixNewlines(singleline map[*Node]bool) {
	// Flat objects aren't a visual unit; their fields are the parent's fields, so they follow the parent's layout.
	// Format() handles the root's singleline check.
	if singleline[n] && !n.Kind().Flat() {
		return
	}
	if (n.Kind() == "{" || n.Value.s == "map(") && len(n.Children) > 0 && n.Post == "" {
		n.Post = "\n"
	}
	for _, ch := range n.Children {
		ch.fixNewlines(singleline)
		if n.Kind().Struct() || n.Kind() == ":[" && n.Name != "" {
			if ch.Kind().Scalar() && ch.Post == "" {
				ch.Post = "\n"
			} else if ch.Kind().Opening() && ch.PostClose == "" {
				ch.PostClose = "\n"
			}
		}
	}
	if n.Value.s != "map(" {
		return
	}
	for i := 0; i+1 < len(n.Children); i += 2 {
		if n.Children[i].Post != "" && n.Children[i].Post != "\n" {
			n.Children[i].Pre = append(n.Children[i].Pre, n.Children[i].Post)
		}
		n.Children[i].Pre = append(n.Children[i].Pre, n.Children[i+1].Pre...)
		n.Children[i].Post, n.Children[i+1].Pre = "", nil
		if n.Children[i+1].Kind().Scalar() && n.Children[i+1].Post == "" {
			n.Children[i+1].Post = "\n"
		} else if n.Children[i+1].Kind().Opening() && n.Children[i+1].PostClose == "" {
			n.Children[i+1].PostClose = "\n"
		}
	}
}

func reindentMultilineString(t Token, indent int) Token {
	st, err := t.As[StringToken]()
	if err != nil || len(st.Quote) <= 1 || st.Quote[len(st.Quote)-1] == '|' {
		return t
	}
	ln := strings.LastIndexByte(st.Quote, '\n')
	if ln == -1 {
		return t
	}
	oldIndent := st.Quote[ln+1 : len(st.Quote)]
	newIndent := strings.Repeat("\t", indent)
	if oldIndent == newIndent {
		return t
	}
	st.Quote = st.Quote[:ln+1] + newIndent
	return MakeToken(st)
}

func appendIndent(ts []string, tabs int) []string {
	if len(ts) > 0 && !Kind(ts[len(ts)-1]).Trivia() {
		return ts
	}
	for range tabs {
		ts = append(ts, "\t")
	}
	return ts
}

func fixComment(s string) string {
	if s == "" || s[0] != '#' || s[len(s)-1] == '\n' {
		return s
	}
	return s + "\n"
}

// tokenizeForFormat converts the node back to its raw tokens, including indentation.
func (n *Node) tokenizeForFormat(ts []string, fieldpath []byte, indent int, singleline map[*Node]bool) []string {
	for _, t := range n.Pre {
		if t == "\n" {
			ts = append(ts, t)
			continue
		}
		ts = append(appendIndent(ts, indent), fixComment(t))
	}
	if !n.Kind().Flat() {
		ts = appendIndent(ts, indent)
		if len(fieldpath) > 0 {
			ts = append(ts, string(append(fieldpath, ':')))
		}
		ts = append(ts, n.Value.s)
	}
	if !n.Kind().Flat() && n.Post != "" {
		ts = append(ts, fixComment(n.Post))
	}
	fp := fieldpath[len(fieldpath):]
	if n.Kind().Flat() {
		indent--
		fp = fieldpath
		if n.Kind().List() && len(fp) > 0 {
			fp = append(fp, '+')
		}
	}

	// Don't double indent the number line in cases like "SomeList [[\n\t1 2]]".
	noindent := !n.Kind().Flat() && len(n.Children) > 0 && n.Post == ""
	if noindent {
		lch := n.Children[len(n.Children)-1]
		noindent = lch.Kind().Opening() && lch.PostClose == ""
		for _, ch := range n.Children[:len(n.Children)-1] {
			noEOLValue := ch.Kind().Scalar() && ch.Post == ""
			noEOLBlock := ch.Kind().Opening() && ch.PostClose == ""
			noindent = noindent && singleline[ch] && (noEOLValue || noEOLBlock)
		}
		if noindent {
			indent--
		}
	}

	if !n.Kind().Struct() {
		for _, ch := range n.Children {
			fp := fp
			if ch.Name != "" {
				fp = append(fp, []byte(ch.Name)...)
			}
			ts = ch.tokenizeForFormat(ts, fp, indent+1, singleline)
		}
	} else {
		for _, ch := range n.Children {
			fp := fp
			if len(fp) > 0 {
				fp = append(fp, '.')
			}
			if ch.Name == "" {
				fp = append(fp, []byte("__MissingNameError__")...)
			}
			for i, ch := range ch.Name {
				if i == 0 && unicode.IsDigit(ch) || !(ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch)) {
					fp = append(fp, '_')
				} else {
					fp = utf8.AppendRune(fp, ch)
				}
			}
			ts = ch.tokenizeForFormat(ts, fp, indent+1, singleline)
		}
	}

	for _, t := range n.PreClose {
		if t != "\n" {
			ts = appendIndent(ts, indent+1)
		}
		ts = append(ts, fixComment(t))
	}
	if n.Kind().Nested() {
		ts = appendIndent(ts, indent)
		ts = append(ts, string(n.Kind().ComplementByte()))
		if n.PostClose != "" {
			ts = append(ts, fixComment(n.PostClose))
		}
	}
	return ts
}

// String returns a compact, one-line string representation without any comments or other fluff.
func (n *Node) String() string {
	if n == nil {
		return ""
	}
	ts := n.tokenizeForString(nil, make([]byte, 0, 1024))
	lastk := Kind("{")
	w := &strings.Builder{}
	for _, t := range ts {
		if !lastk.Name() && !(lastk.Opening() || Kind(t).Closing()) {
			w.WriteByte(' ')
		}
		w.WriteString(t)
		lastk = Kind(t)
	}
	return w.String()
}

func (n *Node) tokenizeForString(ts []string, fieldpath []byte) []string {
	// Print the name if non-flat nodes.
	if !n.Kind().Flat() {
		if len(fieldpath) > 0 {
			ts = append(ts, string(append(fieldpath, ':')))
		}
		if n.Kind().String() {
			s, err := n.Value.As[string]()
			if err != nil {
				ts = append(ts, "error(", "sdin.InvalidRawString:", strconv.Quote(err.Error()), ")")
			} else {
				ts = append(ts, strconv.Quote(s))
			}
		} else {
			ts = append(ts, n.Value.s)
		}
	}

	// Extend the fieldpath for flat nodes.
	fp := fieldpath[len(fieldpath):]
	if n.Kind().Flat() {
		fp = fieldpath
		if n.Kind().List() && len(fp) > 0 {
			fp = append(fp, '+')
		}
	}

	// Print the child nodes.
	if !n.Kind().Struct() {
		for _, ch := range n.Children {
			fp := fp
			if ch.Name != "" {
				fp = append(fp, []byte(ch.Name)...)
			}
			ts = ch.tokenizeForString(ts, fp)
		}
	} else {
		for _, ch := range n.Children {
			fp := fp
			if len(fp) > 0 {
				fp = append(fp, '.')
			}
			if ch.Name == "" {
				fp = append(fp, []byte("__MissingNameError__")...)
			}
			for i, ch := range ch.Name {
				if i == 0 && unicode.IsDigit(ch) || !(ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch)) {
					fp = append(fp, '_')
				} else {
					fp = utf8.AppendRune(fp, ch)
				}
			}
			ts = ch.tokenizeForString(ts, fp)
		}
	}

	// Print the closing token.
	if n.Kind().Nested() {
		ts = append(ts, string(n.Kind().ComplementByte()))
	}
	return ts
}
