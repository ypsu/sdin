package sdin

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ypsu/sdin/tokens"
)

// BareToken represents any bare token.
// Bare tokens are all the raw sdin tokens that don't contain the :, [, ], {, }, (, ), #, ', ", space, tab, and newline symbols.
type BareToken string

// Identifier represents identifier tokens.
// Identifiers are bare tokens that start with a letter, and contain only letters, numbers, and underscores.
// Note that strings are not considered as identifiers.
type Identifier string

// StringToken represents string tokens losslessly.
type StringToken struct {
	// Opening quote, includes the tag and everything up to the first | or \n for multiquote strings.
	// If empty when encoding, a quote will be automatically picked via some heuristics.
	// In the multiquote case the number of quotes will be increased as necessary to encode Data correctly.
	// In the multiline case this contains the indentation string after the newline.
	Quote string

	// The string itself with indentation removed for multiline strings.
	Data string
}

// Tag extracts the raw string's tag from its quote.
// Example:
//
//	The tag of '''some tag\n\t\t|...''' is "some tag".
func (st StringToken) Tag() string {
	t := st.Quote
	if len(t) <= 1 {
		return ""
	}
	q, t := t[0], t[1:]
	for len(t) > 0 && t[0] == q {
		t = t[1:]
	}
	e := 0
	for e < len(t) && t[e] != '\n' && t[e] != '\r' && t[e] != '|' {
		e++
	}
	return t[:e]
}

// MakeToken creates an SDIN bare token to represent v.
// Uses IntoSDINToken for types implementing that.
// Integers, floats, times, durations become numeric tokens.
// [Identifier] values become identifier tokens.
// It stringifies the rest and converts them to sdin strings.
// If the string contains newlines then it becomes a multiline string otherwise a quoted string.
// It panics if used with the wrong type or invalid value (e.g. empty Identifier).
func MakeToken(v any) Token { return Token{tokenFrom(v)} }

// Tokenizable returns true for values that MakeToken can convert to a token.
// For any other value MakeToken will panic.
func Tokenizable(v any) bool {
	if v == nil {
		return true
	}
	switch v.(type) {
	case intoToken:
		return true
	case Token, BareToken, Identifier, StringToken, time.Duration, time.Time:
		return true
	case bool, []byte, string, float32, float64, int8, int16, int32, int64, int, uint8, uint16, uint32, uint64, uint:
		return true
	}
	t := reflect.TypeOf(v)
	pt := reflect.PointerTo(t)
	if pt.Implements(reflect.TypeFor[intoToken]()) || pt.Implements(reflect.TypeFor[intoNode]()) {
		return false
	}
	return builtinType(t) != nil
}

// As parses an sdin token into a T value.
// Implement Interface.FromSDINToken to add support for custom types.
// Returns a wrapped ErrUnsupportedTokenIntoType if the target type doesn't support parsing sdin tokens.
func (t Token) As[T any]() (T, error) {
	var dst T
	err := into(&dst, t.s)
	return dst, err
}

func tokenFrom(v any) string {
	if v == nil {
		return "nil"
	}
	switch tv := v.(type) {
	case intoToken:
		return tv.IntoSDINToken().s

	case Token:
		return tv.s

	case BareToken:
		if tv == "" {
			panic("sdin.TokenFromEmptyBareToken")
		}
		return string(tv)

	case Identifier:
		if tv == "" {
			panic("sdin.TokenFromEmptyIdentifier")
		}
		return string(tv)

	case StringToken:
		if (tv.Quote == "" || tv.Quote == "'") && !hasSpecialByte(tv.Data) {
			return "'" + tv.Data + "'"
		}
		if tv.Quote == "\"" {
			return fmt.Sprintf("%q", tv.Data)
		}
		quotes := 0
		for quotes < len(tv.Quote) && tv.Quote[quotes] == '\'' {
			quotes++
		}
		quotes = max(quotes, 3)
		run := 0
		for _, ch := range tv.Quote[min(len(tv.Quote), quotes):] {
			if ch == '\'' {
				run, quotes = run+1, max(quotes, run+2)
			} else {
				run = 0
			}
		}
		run = 0
		for _, ch := range tv.Data {
			if ch == '\'' {
				run, quotes = run+1, max(quotes, run+2)
			} else {
				run = 0
			}
		}
		q := strings.Repeat("'", quotes)
		if strings.IndexByte(tv.Quote, '|') != -1 || (len(tv.Data) > 0 && tv.Data[len(tv.Data)-1] != '\n') {
			return fmt.Sprintf("%s%s|%s%s", q, tv.Tag(), tv.Data, q)
		}
		w := &strings.Builder{}
		w.WriteString(q)
		w.WriteString(tv.Tag())
		w.WriteByte('\n')
		indent := ""
		if nl := strings.IndexByte(tv.Quote, '\n'); nl != -1 {
			indent = tv.Quote[nl+1:]
		}
		for _, line := range strings.SplitAfter(tv.Data, "\n") {
			if line != "" && line != "\n" {
				w.WriteString(indent)
			}
			w.WriteString(line)
		}
		w.WriteString(indent)
		w.WriteString(q)
		return w.String()

	case bool:
		if tv {
			return "true"
		}
		return "false"

	case []byte:
		if !hasSpecialByte(tv) {
			return fmt.Sprintf("'%s'", tv)
		}
		return fmt.Sprintf("%q", tv)

	case string:
		if !hasSpecialByte(tv) {
			return fmt.Sprintf("'%s'", tv)
		}
		return fmt.Sprintf("%q", tv)

	case float32:
		return strconv.FormatFloat(float64(tv), 'g', -1, 32)

	case float64:
		return strconv.FormatFloat(tv, 'g', -1, 64)

	case int:
		return tokenFromInt64(int64(tv))

	case int8:
		return tokenFromInt64(int64(tv))

	case int16:
		return tokenFromInt64(int64(tv))

	case int32:
		return tokenFromInt64(int64(tv))

	case int64:
		return tokenFromInt64(tv)

	case uint:
		return tokenFromUint64(uint64(tv))

	case uint8:
		return tokenFromUint64(uint64(tv))

	case uint16:
		return tokenFromUint64(uint64(tv))

	case uint32:
		return tokenFromUint64(uint64(tv))

	case uint64:
		return tokenFromUint64(tv)

	case time.Duration:
		return tokenFromDuration(tv)

	case time.Time:
		return tokenFromTime(tv)
	}

	t := reflect.TypeOf(v)
	pt := reflect.PointerTo(t)
	if pt.Implements(reflect.TypeFor[intoToken]()) {
		panic(fmt.Sprintf("sdin.UnsupportedTokenFromNonPointer type=%T: pass a pointer, that one implements IntoSDINToken", v))
	}
	if pt.Implements(reflect.TypeFor[intoNode]()) {
		panic(fmt.Sprintf("sdin.UnsupportedTokenFromIntoSDINNodeType type=%T: convert this into a Node, not Token", v))
	}
	if bt := builtinType(t); bt != nil {
		return tokenFrom(reflect.ValueOf(v).Convert(bt).Interface())
	}
	panic(fmt.Sprintf("sdin.UnsupportedTokenFrom type=%T", v))
}

func hasSpecialByte[T interface{ []byte | string }](v T) bool {
	for i := 0; i < len(v); i++ {
		if v[i] < 32 || v[i] == '\'' {
			return true
		}
	}
	return false
}

func tokenFromInt64(v int64) string {
	if v == 0 {
		return "0"
	}
	unitName, unitMultiplier := "", int64(1)
	for u, m := range units {
		// Skip the time units, and K and Ki too, so that year 2000 or port 8000 remains a normal number.
		if u == "" || u[0] == 'K' || u[0] > 'Z' || m < unitMultiplier || v%m != 0 {
			continue
		}
		unitName, unitMultiplier = u, m
	}
	return strconv.FormatInt(v/unitMultiplier, 10) + unitName
}

func tokenFromUint64(v uint64) string {
	if v == 0 {
		return "0"
	}
	unitName, unitMultiplier := "", uint64(1)
	for u, m := range units {
		m := uint64(m)
		// Skip the time units, and K and Ki too, so that year 2000 or port 8000 remains a normal number.
		if u == "" || u[0] == 'K' || u[0] > 'Z' || m < unitMultiplier || v%m != 0 {
			continue
		}
		unitName, unitMultiplier = u, m
	}
	return strconv.FormatUint(v/unitMultiplier, 10) + unitName
}

func tokenFromDuration(v time.Duration) string {
	if v == 0 {
		return "0s"
	}
	ns, unitName, unitMultiplier := v.Nanoseconds(), "", int64(1)
	for u, m := range units {
		m *= 1e6
		if u == "" || u[0] < 'a' || m < unitMultiplier || ns%m != 0 {
			continue
		}
		unitName, unitMultiplier = u, m
	}
	if unitMultiplier != 1 {
		return strconv.FormatInt(ns/unitMultiplier, 10) + unitName
	}
	return strconv.FormatFloat(float64(ns)/1e9, 'g', -1, 64) + "s"
}

func tokenFromTime(v time.Time) string {
	tzpart := ""
	switch loc := v.Location(); {
	case loc == time.UTC:
	case loc == time.Local:
		tzpart = "@local"
	case loc.String() == "":
		v = v.UTC()
	default:
		tzpart = "@" + loc.String()
	}
	if v.Hour() == 0 && v.Minute() == 0 && v.Second() == 0 && v.Nanosecond() == 0 {
		return v.Format("2006-01-02") + tzpart
	}
	if v.Second() == 0 && v.Nanosecond() == 0 {
		return v.Format("2006-01-02.1504") + tzpart
	}
	if v.Nanosecond() == 0 {
		return v.Format("2006-01-02.150405") + tzpart
	}
	if v.Nanosecond()%1_000_000 == 0 {
		return v.Format("2006-01-02.150405.000") + tzpart
	}
	if v.Nanosecond()%1_000 == 0 {
		return v.Format("2006-01-02.150405.000000") + tzpart
	}
	return v.Format("2006-01-02.150405.000000000") + tzpart
}

// ErrUnsupportedTokenIntoType means that the type passed to [Token.As] doesn't implement the [Interface.FromSDINToken] function.
var ErrUnsupportedTokenIntoType = errors.New("sdin.UnsupportedTokenIntoType")

type intoInteger interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64
}

func convertIntoInt[To intoInteger](dst *To, s string) error {
	v, err := intoInt64(s)
	if err != nil {
		return err
	}
	*dst = To(v)
	if int64(*dst) != v {
		return fmt.Errorf("sdin.IntOutOfRange s=%s type=%T", s, *dst)
	}
	return nil
}

func convertIntoUint[To intoInteger](dst *To, s string) error {
	v, err := intoUint64(s)
	if err != nil {
		return err
	}
	*dst = To(v)
	if uint64(*dst) != v {
		return fmt.Errorf("sdin.UintOutOfRange s=%s type=%T", s, *dst)
	}
	return nil
}

func into(r any, s string) error {
	if s == "" {
		return fmt.Errorf("sdin.EmptyToken")
	}
	switch ptr := r.(type) {
	case fromToken:
		if err := ptr.FromSDINToken(Token{s}); err != nil {
			return fmt.Errorf("sdin.FromSDINToken type=%T: %v", ptr, err)
		}
		return nil
	case *Token:
		*ptr = Token{s}
		return nil
	case *BareToken:
		k := Kind(s)
		if k.String() {
			return fmt.Errorf("sdin.StringAsBareToken")
		}
		if !k.Scalar() {
			return fmt.Errorf("sdin.NonScalarAsBareToken")
		}
		*ptr = BareToken(s)
		return nil
	case *Identifier:
		for i, ch := range s {
			if i == 0 && (ch == '\'' || ch == '"') {
				return fmt.Errorf("sdin.StringAsIdentifier s=%q: strings are not considered identifiers", s)
			}
			if i == 0 && ch != '_' && !unicode.IsLetter(ch) {
				return fmt.Errorf("sdin.IdentifierWithNonLetterStart firstchar=%q s=%q: identifier must start with a letter or underscore", ch, s)
			}
			if ch != '_' && !unicode.IsLetter(ch) && !unicode.IsNumber(ch) {
				return fmt.Errorf("sdin.NonIdentifierCharacter i=%d ch=%q s=%q: identifier must contain only letters, numbers, or underscores", i, ch, s)
			}
		}
		*ptr = Identifier(s)
		return nil
	case *StringToken:
		p, err := intoStringToken(s)
		*ptr = p
		return err
	case *bool:
		switch s {
		case "false":
			*ptr = false
		case "true":
			*ptr = true
		default:
			return fmt.Errorf("sdin.NonBoolToken token=%q: bools must be either true or false", s)
		}
		return nil
	case *[]byte:
		p, err := intoString(s)
		*ptr = []byte(p)
		return err
	case *string:
		s, err := intoString(s)
		*ptr = s
		return err
	case *float32:
		f, err := intoFloat(s, 32)
		*ptr = float32(f)
		return err
	case *float64:
		f, err := intoFloat(s, 64)
		*ptr = f
		return err
	case *int:
		return convertIntoInt(ptr, s)
	case *int8:
		return convertIntoInt(ptr, s)
	case *int16:
		return convertIntoInt(ptr, s)
	case *int32:
		return convertIntoInt(ptr, s)
	case *int64:
		return convertIntoInt(ptr, s)
	case *uint:
		return convertIntoUint(ptr, s)
	case *uint8:
		return convertIntoUint(ptr, s)
	case *uint16:
		return convertIntoUint(ptr, s)
	case *uint32:
		return convertIntoUint(ptr, s)
	case *uint64:
		return convertIntoUint(ptr, s)
	case *time.Duration:
		if strings.IndexByte("smhdw", s[len(s)-1]) == -1 {
			return fmt.Errorf("sdin.NotADuration s=%q (duration must have a s,m,h,d,w unit)", s)
		}
		f, err := intoFloat(s, 64)
		*ptr = time.Duration(math.Round(f * float64(time.Millisecond)))
		return err
	case *time.Time:
		if len(s) < 10 || s[4] != '-' || s[7] != '-' {
			return fmt.Errorf("sdin.NotADateTime s=%q", s)
		}
		v, err := intoTime(s)
		*ptr = v
		return err
	case *tokens.JSONValue:
		v, err := intoJSONValue(s)
		*ptr = v
		return err
	default:
		if rv := reflect.ValueOf(r); rv.Kind() == reflect.Pointer && !rv.IsNil() {
			if bt := builtinType(rv.Type().Elem()); bt != nil && bt != rv.Type().Elem() {
				return into(rv.Convert(reflect.PointerTo(bt)).Interface(), s)
			}
		}
		return fmt.Errorf("%w target=%T (dst must be a pointer to a supported builtin type or implement sdin.FromSDINToken)", ErrUnsupportedTokenIntoType, r)
	}
}

func intoString(s string) (string, error) {
	st, err := intoStringToken(s)
	if err != nil {
		return "", fmt.Errorf("sdin.ParseStringToken: %v", err)
	}
	return st.Data, nil
}

func intoStringToken(s string) (StringToken, error) {
	// Handle the literal and quoted strings.
	if !Kind(s).String() {
		if Kind(s).Scalar() {
			return StringToken{}, fmt.Errorf("sdin.ParseBareAsString s=%q: bare tokens are not considered as strings", s)
		}
		return StringToken{}, fmt.Errorf("sdin.ParseNonStringString token=%q", s)
	}
	if len(s) <= 1 || s[0] != s[len(s)-1] {
		return StringToken{}, fmt.Errorf("sdin.ParseUnclosedString s=%q", s)
	}
	if s[0] == '\'' && (len(s) == 2 || s[1] != '\'') {
		return StringToken{Quote: s[:1], Data: s[1 : len(s)-1]}, nil
	}
	if s[0] == '"' && (len(s) == 2 || s[1] != '"') {
		var p string
		if _, err := fmt.Sscanf(s, "%q", &p); err != nil {
			return StringToken{}, fmt.Errorf("sdin.ParseQuotedString s=%q: %v", s, err)
		}
		return StringToken{Quote: s[:1], Data: p}, nil
	}

	// Find the boundary and the type marker.
	if s[0] != '\'' {
		return StringToken{}, fmt.Errorf("sdin.InvalidMultiquoteString quote=%q: quote must be '", s[0])
	}
	quotes := 1
	for quotes < len(s) && s[quotes] == s[0] {
		quotes++
	}
	for i := range quotes {
		if s[len(s)-i-1] != '\'' {
			return StringToken{}, fmt.Errorf("sdin.MultiquoteStringWithoutMatchingQuotes openquotes=%d closequotes=%d", quotes, i)
		}
	}
	startmarkpos := quotes
	for startmarkpos < len(s) && (s[startmarkpos] != '\n' && s[startmarkpos] != '|') {
		startmarkpos++
	}
	if startmarkpos == len(s) {
		return StringToken{}, fmt.Errorf("sdin.MultiquoteStringWithoutStartmark: must have \\n or | to mark the start")
	}
	startmark := s[startmarkpos]

	// Handle the raw strings.
	if startmark == '|' {
		return StringToken{Quote: s[:startmarkpos+1], Data: s[startmarkpos+1 : len(s)-quotes]}, nil
	}
	endmarkpos := strings.LastIndexByte(s, '\n')

	// Handle the multiline strings by deindenting them.
	for i := len(s) - quotes - 1; i > endmarkpos; i-- {
		if s[i] != ' ' && s[i] != '\t' {
			return StringToken{}, fmt.Errorf("sdin.MultilineStringWithNonemptyCloser got=%q: the close quote line must contain only the quotes", s[endmarkpos+1:len(s)-quotes])
		}
	}
	indent := s[endmarkpos+1 : len(s)-quotes]
	r, p, nn, line := &strings.Builder{}, startmarkpos+1, strings.IndexByte(s[startmarkpos+1:], '\n'), 1
	r.Grow(len(s) - 2*quotes - len(indent))
	for nn != -1 {
		if strings.HasPrefix(s[p:], indent) {
			p, nn = p+len(indent), nn-len(indent)
		} else if s[p] != '\n' {
			return StringToken{}, fmt.Errorf("sdin.InconsistentMultilineStringIndent line=%d got=%q want=%q", line, s[p:min(len(s), p+len(indent))], indent)
		}
		r.WriteString(s[p : p+nn+1])
		p, line = p+nn+1, line+1
		nn = strings.IndexByte(s[p:], '\n')
	}
	quote := s[:startmarkpos+1+len(indent)]
	if s[startmarkpos+1:startmarkpos+1+len(indent)] != indent {
		quote = s[:startmarkpos+1] + indent
	}
	return StringToken{Quote: quote, Data: r.String()}, nil
}

var units = map[string]int64{
	"":   1,
	"K":  1e3,
	"M":  1e6,
	"G":  1e9,
	"T":  1e12,
	"P":  1e15,
	"Ki": 1 << 10,
	"Mi": 1 << 20,
	"Gi": 1 << 30,
	"Ti": 1 << 40,
	"Pi": 1 << 50,
	"s":  1000,
	"m":  1000 * 60,
	"h":  1000 * 60 * 60,
	"d":  1000 * 60 * 60 * 24,
	"w":  1000 * 60 * 60 * 24 * 7,
}

func intoFloat(s string, precision int) (float64, error) {
	if c := s[0]; c != '+' && c != '-' && !('0' <= c && c <= '9') {
		return 0, fmt.Errorf("sdin.NotANumberToken value=%q", s)
	}

	// Time and date.
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		t, err := intoTime(s)
		return float64(t.UnixMicro()) / 1000, err
	}

	s = strings.ReplaceAll(s, "_", "")
	unitless, unit := s, ""
	for i := len(s) - 1; 'a' <= s[i] && s[i] <= 'z' || 'A' <= s[i] && s[i] <= 'Z'; i-- {
		unitless, unit = s[:i], s[i:]
	}
	unitmult, found := units[unit]
	if !found {
		return 0.0, fmt.Errorf("sdin.UnsupportedNumberUnit unit=%s token=%q", unit, s)
	}

	f, err := strconv.ParseFloat(unitless, precision)
	if err != nil {
		return 0.0, fmt.Errorf("sdin.ParseFloat: %v", err)
	}
	r := f * float64(unitmult)
	if precision == 32 && math.IsInf(float64(float32(r)), 0) {
		return 0.0, fmt.Errorf("sdin.Float32Overflow s=%s", s)
	}
	return r, nil
}

func signedMulOverflows(a, b int64) bool {
	if a == 0 || b == 0 || a == 1 || b == 1 {
		return false
	}
	if a == math.MinInt64 || b == math.MinInt64 {
		return true
	}
	c := a * b
	return c/b != a
}

func unsignedMulOverflows(a, b uint64) bool {
	if a == 0 || b == 0 || a == 1 || b == 1 {
		return false
	}
	c := a * b
	return c/b != a
}

var timeFormats = []string{
	"2006-01-02",
	"2006-01-02.1504",
	"2006-01-02.150405",
}

func intoTime(s string) (time.Time, error) {
	tz := time.UTC
	tmpart, tzpart, ok := strings.Cut(s, "@")
	tmpart = strings.ReplaceAll(tmpart, "_", "")
	if ok && tzpart == "local" {
		tz = time.Local
	} else if ok {
		loc, err := time.LoadLocation(tzpart)
		if err != nil {
			return time.Time{}, fmt.Errorf("sdin.LoadTimezone value=%q: %v", s, err)
		}
		tz = loc
	}
	for _, f := range timeFormats {
		t, err := time.ParseInLocation(f, tmpart, tz)
		if err == nil { // on success
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("sdin.UnsupportedTimeFormat value=%q", s)
}

func intoInt64(s string) (int64, error) {
	if c := s[0]; c != '+' && c != '-' && !('0' <= c && c <= '9') {
		return 0, fmt.Errorf("sdin.NotANumberToken value=%q", s)
	}

	// Time and date.
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		t, err := intoTime(s)
		return t.UnixMilli(), err
	}

	nus := strings.ReplaceAll(s, "_", "") // No-UnderScore token

	// Numbers in different number base.
	if len(s) >= 3 && s[0] == '0' {
		base := 0
		if s[1] == 'b' {
			base = 2
		} else if s[1] == 'o' {
			base = 8
		} else if s[1] == 'x' {
			base = 16
		}
		if base != 0 {
			if v, err := strconv.ParseInt(nus[2:], base, 64); err == nil { // on success
				return v, nil
			}
		}
	}

	unitless, unit := nus, ""
	for i := len(nus) - 1; 'a' <= nus[i] && nus[i] <= 'z' || 'A' <= nus[i] && nus[i] <= 'Z'; i-- {
		unitless, unit = nus[:i], nus[i:]
	}
	unitmult, found := units[unit]
	if !found {
		return 0, fmt.Errorf("sdin.UnsupportedNumberUnit unit=%s token=%q", unit, s)
	}

	v, intErr := strconv.ParseInt(unitless, 10, 64)
	if intErr != nil {
		f, err := strconv.ParseFloat(unitless, 64)
		if err != nil {
			return 0, fmt.Errorf("sdin.ParseInt: %v", intErr)
		}
		f *= float64(unitmult)
		if f != math.Trunc(f) {
			return 0, fmt.Errorf("sdin.FractionalFloatAsInteger s=%v: floats with nonzero fractional part cannot be converted to an integer without loss", nus)
		}
		const maxExactFloat = 1<<53 - 1
		if f < -maxExactFloat || maxExactFloat < f {
			return 0, fmt.Errorf("sdin.LargeFloatAsInteger s=%v: this number is outside the safe integer range of floats (-2^53, 2^53)", nus)
		}
		return int64(f), nil
	}
	if signedMulOverflows(v, unitmult) {
		return 0, fmt.Errorf("sdin.NumTooLarge v=%s", s)
	}
	return v * unitmult, nil
}

func intoUint64(s string) (uint64, error) {
	if c := s[0]; !('0' <= c && c <= '9') {
		return 0, fmt.Errorf("sdin.NotANumberToken value=%q", s)
	}
	nus := strings.ReplaceAll(s, "_", "") // No-UnderScore token

	// Numbers in different number base.
	if len(s) >= 3 && s[0] == '0' {
		base := 0
		if s[1] == 'b' {
			base = 2
		} else if s[1] == 'o' {
			base = 8
		} else if s[1] == 'x' {
			base = 16
		}
		if base != 0 {
			if v, err := strconv.ParseUint(nus[2:], base, 64); err == nil { // on success
				return v, nil
			}
		}
	}

	unitless, unit := nus, ""
	for i := len(nus) - 1; 'a' <= nus[i] && nus[i] <= 'z' || 'A' <= nus[i] && nus[i] <= 'Z'; i-- {
		unitless, unit = nus[:i], nus[i:]
	}
	unitmult := uint64(units[unit])
	if unitmult == 0 {
		return 0, fmt.Errorf("sdin.UnsupportedNumberUnit unit=%s token=%q", unit, s)
	}

	v, intErr := strconv.ParseUint(unitless, 10, 64)
	if intErr != nil {
		f, err := strconv.ParseFloat(unitless, 64)
		if err != nil {
			return 0, fmt.Errorf("sdin.ParseUint: %v", intErr)
		}
		f *= float64(unitmult)
		if f != math.Trunc(f) {
			return 0, fmt.Errorf("sdin.FractionalFloatAsInteger s=%v: floats with nonzero fractional part cannot be converted to an integer without loss", nus)
		}
		const maxExactFloat = 1<<53 - 1
		if maxExactFloat < f {
			return 0, fmt.Errorf("sdin.LargeFloatAsInteger s=%v: this number is outside the safe integer range of floats (-2^53, 2^53)", nus)
		}
		return uint64(f), nil
	}
	if unsignedMulOverflows(v, unitmult) {
		return 0, fmt.Errorf("sdin.UnsignedNumTooLarge v=%s", s)
	}
	return v * unitmult, nil
}

func intoJSONValue(s string) (tokens.JSONValue, error) {
	k := Kind(s)
	switch {
	case k.String():
		s, err := Token{s}.As[string]()
		if err != nil {
			return "", fmt.Errorf("sdin.ParseStringForJSONValue: %v", err)
		}
		js, _ := json.Marshal(s)
		return tokens.JSONValue(js), nil
	case k.Scalar():
		if s == "nil" {
			return tokens.JSONValue("null"), nil
		}
		if fs := strings.TrimPrefix(strings.ReplaceAll(s, "_", ""), "+"); json.Valid([]byte(fs)) {
			return tokens.JSONValue(fs), nil
		}
		i, err := Token{s}.As[int64]()
		if err == nil { // on success
			return tokens.JSONValue(fmt.Sprintf("%d", i)), nil
		}
		f, err := Token{s}.As[float64]()
		if err != nil {
			return "", fmt.Errorf("sdin.ParseJSONFloat: %v", err)
		}
		return tokens.JSONValue(fmt.Sprintf("%f", f)), nil
	default:
		return "", fmt.Errorf("sdin.UnsupportedTokenKindForJSONValue token=%q", s)
	}
}

var kindTypes = map[reflect.Kind]reflect.Type{
	reflect.Bool:    reflect.TypeFor[bool](),
	reflect.String:  reflect.TypeFor[string](),
	reflect.Int:     reflect.TypeFor[int](),
	reflect.Int8:    reflect.TypeFor[int8](),
	reflect.Int16:   reflect.TypeFor[int16](),
	reflect.Int32:   reflect.TypeFor[int32](),
	reflect.Int64:   reflect.TypeFor[int64](),
	reflect.Uint:    reflect.TypeFor[uint](),
	reflect.Uint8:   reflect.TypeFor[uint8](),
	reflect.Uint16:  reflect.TypeFor[uint16](),
	reflect.Uint32:  reflect.TypeFor[uint32](),
	reflect.Uint64:  reflect.TypeFor[uint64](),
	reflect.Float32: reflect.TypeFor[float32](),
	reflect.Float64: reflect.TypeFor[float64](),
}

func builtinType(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Slice && t.Elem() == reflect.TypeFor[byte]() {
		return reflect.TypeFor[[]byte]()
	}
	return kindTypes[t.Kind()]
}
