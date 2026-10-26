// Package tokens provides internal helper functions for working with sdin data.
package tokens

import (
	"fmt"
	"strings"
)

// TSlice represents start, end of a token slice from an unspecified buffer.
// Line describes the line number this slice is from.
type TSlice struct {
	Start, End int
	Line       int
}

// Tokenize returns the start+end offsets for each token.
func Tokenize(p string) ([]TSlice, error) {
	starts := func(ch byte) bool {
		return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' || ch == '{' || ch == '}' || ch == '[' || ch == ']' || ch == '(' || ch == ')' || ch == '#' || ch == ':' || ch == '\'' || ch == '"'
	}
	var (
		ts      []TSlice
		n, line int = len(p), 1
		i, j    int
	)

	for i < n {
		c := p[i]
		switch {
		// Whitespace.
		case c == ' ' || c == '\t' || c == '\r':
			j = i
			for j < n && (p[j] == ' ' || p[j] == '\t' || p[j] == '\r') {
				j++
			}
			i = j

		// Newline.
		case c == '\n':
			i, line, ts = i+1, line+1, append(ts, TSlice{i, i + 1, line})

		// Struct or list.
		case strings.IndexByte("{}[])", c) != -1:
			i, ts = i+1, append(ts, TSlice{i, i + 1, line})

		// Comment.
		case c == '#':
			j = i + 1
			for j < n && p[j-1] != '\n' {
				j++
			}
			i, line, ts = j, line+1, append(ts, TSlice{i, j, line})

		// Multiquote string.
		case i+2 < n && c == '\'' && p[i+1] == c && p[i+2] == c:
			open := 3
			for i+open < n && p[i+open] == c {
				open++
			}
			j, run, newlines := i+open, 0, 0
			for j < n && (run < open || p[j] == c) {
				if p[j] == c {
					run++
				} else {
					run = 0
				}
				if p[j] == '\n' {
					newlines++
				}
				j++
			}
			if run < open {
				return nil, fmt.Errorf("sdin.UnterminatedMultiquoteString line=%d", line)
			}
			i, line, ts = j, line+newlines, append(ts, TSlice{i, j, line})

		// Literal string.
		case c == '\'':
			j = i + 1
			for j < n && p[j] != '\'' && p[j] != '\n' {
				j++
			}
			if j == n || p[j] == '\n' {
				return nil, fmt.Errorf("sdin.UnterminatedLiteralString line=%d", line)
			}
			i, ts = j+1, append(ts, TSlice{i, j + 1, line})

		// Quoted string.
		case c == '"':
			j, escaping := i+1, false
			for j < n && p[j] != '\n' && (p[j] != '"' || escaping) {
				if !escaping && p[j] == '\\' {
					escaping = true
				} else if escaping {
					escaping = false
				}
				j++
			}
			if j == n || p[j] == '\n' {
				return nil, fmt.Errorf("sdin.UnterminatedQuotedString line=%d", line)
			}
			i, ts = j+1, append(ts, TSlice{i, j + 1, line})

		default:
			// Bare token.
			j = i
			for j < n && !starts(p[j]) {
				j++
			}
			if j < n && (p[j] == '(' || p[j] == ':') {
				j++
			}
			i, ts = j, append(ts, TSlice{i, j, line})
		}
	}
	return ts, nil
}
