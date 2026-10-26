package sdin

import (
	"strings"
	"unicode"
)

// TStruct represents a nested struct.
func TStruct() Token { return Token{"{"} }

// TFlatStruct represents a flat struct.
func TFlatStruct() Token { return Token{":{"} }

// TList represents a nested list.
func TList() Token { return Token{"["} }

// TFlatList represents a flat list.
func TFlatList() Token { return Token{":["} }

// TForm represents a form token with the given name.
func TForm(name string) Token { return Token{name + "("} }

// TFlatForm represents a flat form.
func TFlatForm() Token { return Token{":("} }

// Kind interprets a raw sdin token and makes its properties queriable.
// It's undefined to pass an empty string; the functions might panic.
type Kind string

// Trivia returns true iff the token is a newline or a comment.
func (k Kind) Trivia() bool {
	return k == "\n" || k != "" && k[0] == '#'
}

// Scalar returns true iff the token represents a string or a bare value like an identifier or number.
func (k Kind) Scalar() bool {
	return k != "" && k[0] != '#' && k[0] != '\n' && k[0] != ' ' && k[0] != '\t' && !k.Composite()
}

// Bare returns true iff the token represents a bare value.
func (k Kind) Bare() bool {
	return k.Scalar() && !k.String()
}

// Composite returns true iff the token represents a list, struct, or a form.
func (k Kind) Composite() bool { return k.Opening() || k.Closing() }

// Name returns true iff the token is in the form of `SomeIdentifier:` or `SomeIdentifier+:`.
func (k Kind) Name() bool {
	return len(k) > 0 && k[0] != '#' && k[len(k)-1] == ':'
}

// Opening returns true iff the token represents the opening token for a list, struct, or a form.
func (k Kind) Opening() bool {
	const s = "[{("
	return k != "" && k[0] != '#' && strings.IndexByte(s, k[len(k)-1]) != -1
}

// Closing returns true iff the token represents the closing token for a list, struct, or a form.
func (k Kind) Closing() bool {
	const s = "]})"
	return k != "" && k[0] != '#' && strings.IndexByte(s, k[len(k)-1]) != -1
}

// OpenerByte returns the opener byte of a list, struct, or form.
// Returns one of `[{(` for composite nodes.
func (k Kind) OpenerByte() byte {
	return k[len(k)-1]
}

// ComplementByte returns the complement byte of a list, struct, or form.
// Returns `]})` for `[{(` and vice versa, panics if scalar.
func (k Kind) ComplementByte() byte {
	const a = "[{()}]"
	const b = "]})({["
	return b[strings.IndexByte(a, k[len(k)-1])]
}

// Struct returns true iff the token represents a flat or nested struct.
func (k Kind) Struct() bool {
	return k != "" && k[0] != '#' && k.OpenerByte() == '{'
}

// List returns true iff the token represents a flat or nested list.
func (k Kind) List() bool {
	return k != "" && k[0] != '#' && k.OpenerByte() == '['
}

// Form returns true iff the token is in the form of `SomeIdentifier(` or `somepackage.SomeIdentifier(`.
func (k Kind) Form() bool {
	return k != "" && k[0] != '#' && k.OpenerByte() == '('
}

// FormName returns SomeIdentifier or somepackage.SomeIdentifier for tokens like token `SomeIdentifier(` and `somepackage.SomeIdentifier(`.
// Returns empty string if token is not a form.
func (k Kind) FormName() string {
	if k != "" && k[0] == '#' {
		return ""
	}
	if d, ok := strings.CutSuffix(string(k), "("); ok {
		return d
	}
	return ""
}

// PackageName returns the part before the first . for bare and form tokens.
// E.g. `sdin.KeepSorted(` -> `sdin`.
func (k Kind) PackageName() string {
	if !k.Form() && !(k.Scalar() && !k.String()) {
		return ""
	}
	if i := strings.IndexByte(string(k), '.'); i != -1 {
		return string(k)[:i]
	}
	return ""
}

// Nested returns true iff the token represents a nested list, nested struct, or a form.
func (k Kind) Nested() bool { return k.Opening() && k[0] != ':' }

// Flat returns true iff the token represents a flat list, a flat struct, or a flat form.
func (k Kind) Flat() bool { return k == ":{" || k == ":[" || k == ":(" }

// Identifier returns true iff the token matches the regexp `[A-Za-z_][0-9A-Za-z_]*`.
func (k Kind) Identifier() bool {
	if len(k) == 0 {
		return false
	}
	for i, ch := range k {
		if i == 0 && !unicode.IsLetter(ch) && ch != '_' {
			return false
		}
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
			return false
		}
	}
	return true
}

// IdentifierPath returns true iff the token is a dot-separated sequence of Identifiers.
func (k Kind) IdentifierPath() bool {
	for part := range strings.SplitSeq(string(k), ".") {
		if !Kind(part).Identifier() {
			return false
		}
	}
	return true
}

// String returns true iff the token represents a string.
func (k Kind) String() bool {
	return k != "" && (k[0] == '\'' || k[0] == '"')
}
