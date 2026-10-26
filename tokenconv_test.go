package sdin_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ypsu/efftesting/efft"
	"github.com/ypsu/sdin"
	"github.com/ypsu/sdin/tokens"
)

func into[T any](s string) (T, error) {
	return sdin.MakeTokenFromRepr(s).As[T]()
}

func replaceQuote(s string) string { return strings.ReplaceAll(s, "`", "`") }

type tag struct{ Key, Value string }

func (t *tag) FromSDINToken(token sdin.Token) error {
	k, v, ok := strings.Cut(token.Repr(), "=")
	if !ok {
		return fmt.Errorf("sdintest.TagWithoutSeparator token=%q", token.Repr())
	}
	t.Key, t.Value = k, v
	return nil
}

func (t *tag) IntoSDINToken() sdin.Token {
	return sdin.MakeTokenFromRepr(fmt.Sprintf("%s=%s", t.Key, t.Value))
}

func TestTokenAs(t *testing.T) {
	efft.Init(t)

	efft.Effect(efft.Must1(into[sdin.Token]("hello")).Repr()).Equals("hello")
	efft.Effect(efft.Must1(into[sdin.Token]("'hello'")).Repr()).Equals("'hello'")

	efft.Effect(into[bool]("true")).Equals("true")
	efft.Effect(into[bool]("false")).Equals("false")
	efft.Effect(into[bool]("foo")).Equals("sdin.NonBoolToken token=\"foo\": bools must be either true or false")

	efft.Effect(into[sdin.Identifier]("Hello")).Equals("\"Hello\"")
	efft.Effect(into[sdin.Identifier]("12")).Equals("sdin.IdentifierWithNonLetterStart firstchar='1' s=\"12\": identifier must start with a letter or underscore")
	efft.Effect(into[int64]("Hello")).Equals("sdin.NotANumberToken value=\"Hello\"")
	efft.Effect(into[float64]("Hello")).Equals("sdin.NotANumberToken value=\"Hello\"")
	efft.Effect(into[time.Time]("Hello")).Equals("sdin.NotADateTime s=\"Hello\"")

	efft.Effect(into[string]("''")).Equals("")
	efft.Effect(into[string]("\"\"")).Equals("")
	efft.Effect(into[string]("'''|'''")).Equals("")
	efft.Effect(into[string]("'''tag|'''")).Equals("")
	efft.Effect(into[string]("''''tag with ''' quotes|''''")).Equals("")
	efft.Effect(into[string]("'''tag\n'''")).Equals("")

	efft.Effect(into[string]("'hello  world'")).Equals("hello  world")
	efft.Effect(into[[]byte]("'hello  world'")).Equals("hello  world")
	efft.Effect(into[int64]("'hello  world'")).Equals("sdin.NotANumberToken value=\"'hello  world'\"")
	efft.Effect(into[float64]("'hello  world'")).Equals("sdin.NotANumberToken value=\"'hello  world'\"")
	efft.Effect(into[time.Time]("'hello  world'")).Equals("sdin.NotADateTime s=\"'hello  world'\"")
	efft.Effect(into[string]("\"hello  world\"")).Equals("hello  world")
	efft.Effect(into[string]("'''|hello\n\tworld'''")).Equals(`
		hello
			world`)
	efft.Effect(into[string]("'''tag|hello\n\tworld'''")).Equals(`
		hello
			world`)
	efft.Effect(into[string]("'''tag\n\thello\n\t\tworld\n\t'''")).Equals(`
		hello
			world
		`)
	efft.Effect(into[string](`"\x"`)).Equals("sdin.ParseStringToken: sdin.ParseQuotedString s=\"\\\"\\\\x\\\"\": invalid syntax")

	efft.Effect(into[string]("'hello")).Equals("sdin.ParseStringToken: sdin.ParseUnclosedString s=\"'hello\"")
	efft.Effect(into[string]("\"hello")).Equals("sdin.ParseStringToken: sdin.ParseUnclosedString s=\"\\\"hello\"")
	efft.Effect(into[string]("'''|hello")).Equals("sdin.ParseStringToken: sdin.ParseUnclosedString s=\"'''|hello\"")
	efft.Effect(into[string]("'''\nhello")).Equals("sdin.ParseStringToken: sdin.ParseUnclosedString s=\"'''\\nhello\"")
	efft.Effect(into[string]("'''\n\t\thello\n\tworld\n\t\t'''")).Equals("sdin.ParseStringToken: sdin.InconsistentMultilineStringIndent line=2 got=\"\\tw\" want=\"\\t\\t\"")
	efft.Effect(into[string]("'''|'hello''''")).Equals("'hello'")
	efft.Effect(into[string]("'''|hello''")).Equals("sdin.ParseStringToken: sdin.MultiquoteStringWithoutMatchingQuotes openquotes=3 closequotes=2")
	efft.Effect(into[string]("'''\nhello\nworld'''")).Equals("sdin.ParseStringToken: sdin.MultilineStringWithNonemptyCloser got=\"world\": the close quote line must contain only the quotes")

	efft.Effect(into[sdin.StringToken](`'hello\nworld'`)).Equals(`
		{
		  "Quote": "'",
		  "Data": "hello\\nworld"
		}`,
	)
	efft.Effect(into[sdin.StringToken](`"hello\nworld"`)).Equals(`
		{
		  "Quote": "\"",
		  "Data": "hello\nworld"
		}`,
	)
	efft.Effect(replaceQuote(efft.Stringify(into[sdin.StringToken]("'''|hello'''")))).Equals(`
		{
		  "Quote": "'''|",
		  "Data": "hello"
		}`)
	efft.Effect(replaceQuote(efft.Stringify(into[sdin.StringToken]("'''''|hello'''''")))).Equals(`
		{
		  "Quote": "'''''|",
		  "Data": "hello"
		}`)
	efft.Effect(replaceQuote(efft.Stringify(into[sdin.StringToken]("'''\nhello\n'''")))).Equals(`
		{
		  "Quote": "'''\n",
		  "Data": "hello\n"
		}`)
	efft.Effect(replaceQuote(efft.Stringify(into[sdin.StringToken]("'''\n\t\thello\n\t\t'''")))).Equals(`
		{
		  "Quote": "'''\n\t\t",
		  "Data": "hello\n"
		}`)
	efft.Effect(replaceQuote(efft.Stringify(into[sdin.StringToken]("'''my tag|hello'''")))).Equals(`
		{
		  "Quote": "'''my tag|",
		  "Data": "hello"
		}`)
	efft.Effect(replaceQuote(efft.Stringify(into[sdin.StringToken]("'''my tag\nhello\n'''")))).Equals(`
		{
		  "Quote": "'''my tag\n",
		  "Data": "hello\n"
		}`)
	efft.Effect(replaceQuote(efft.Stringify(into[sdin.StringToken]("'''my tag\n\t\thello\n\t\t'''")))).Equals(`
		{
		  "Quote": "'''my tag\n\t\t",
		  "Data": "hello\n"
		}`)
	efft.Effect(replaceQuote(efft.Stringify(into[sdin.StringToken]("'''my tag\n\n\t\thello\n\t\t'''")))).Equals(`
		{
		  "Quote": "'''my tag\n\t\t",
		  "Data": "\nhello\n"
		}`)

	efft.Effect(efft.Must1(into[sdin.StringToken](`'hello\nworld'`)).Tag()).Equals("")
	efft.Effect(efft.Must1(into[sdin.StringToken](`"hello\nworld"`)).Tag()).Equals("")
	efft.Effect(efft.Must1(into[sdin.StringToken]("'''|hello'''")).Tag()).Equals("")
	efft.Effect(efft.Must1(into[sdin.StringToken]("'''''|hello'''''")).Tag()).Equals("")
	efft.Effect(efft.Must1(into[sdin.StringToken]("'''\nhello\n'''")).Tag()).Equals("")
	efft.Effect(efft.Must1(into[sdin.StringToken]("'''\n\t\thello\n\t\t'''")).Tag()).Equals("")
	efft.Effect(efft.Must1(into[sdin.StringToken]("'''my tag|hello'''")).Tag()).Equals("my tag")
	efft.Effect(efft.Must1(into[sdin.StringToken]("'''my tag\nhello\n'''")).Tag()).Equals("my tag")
	efft.Effect(efft.Must1(into[sdin.StringToken]("'''my tag\n\t\thello\n\t\t'''")).Tag()).Equals("my tag")

	efft.Effect(into[float64]("47")).Equals("47")
	efft.Effect(into[float64]("-123.456e-2")).Equals("-1.23456")
	efft.Effect(into[float64]("0e1")).Equals("0")
	efft.Effect(into[float64]("1.5m")).Equals("90000")
	efft.Effect(into[float64]("1e1000")).Equals("sdin.ParseFloat: strconv.ParseFloat: parsing \"1e1000\": value out of range")

	efft.Effect(into[int64]("0xa0")).Equals("160")
	efft.Effect(into[int64]("0xA0")).Equals("160")
	efft.Effect(into[int64]("0b100")).Equals("4")
	efft.Effect(into[int64]("1e2")).Equals("100")
	efft.Effect(into[int64]("0e2")).Equals("0")
	efft.Effect(into[int64]("0a1")).Equals("sdin.ParseInt: strconv.ParseInt: parsing \"0a1\": invalid syntax")
	efft.Effect(into[int64]("0b102")).Equals("sdin.ParseInt: strconv.ParseInt: parsing \"0b102\": invalid syntax")
	efft.Effect(into[int64]("0o777")).Equals("511")
	efft.Effect(into[float64]("0o777")).Equals("sdin.ParseFloat: strconv.ParseFloat: parsing \"0o777\": invalid syntax")
	efft.Effect(into[int64]("0o777.7")).Equals("sdin.ParseInt: strconv.ParseInt: parsing \"0o777.7\": invalid syntax")
	efft.Effect(into[float64]("0o777.7")).Equals("sdin.ParseFloat: strconv.ParseFloat: parsing \"0o777.7\": invalid syntax")

	efft.Effect(into[int64]("47")).Equals("47")
	efft.Effect(into[int64]("+47")).Equals("47")
	efft.Effect(into[int64]("-47")).Equals("-47")
	efft.Effect(into[int64]("10m")).Equals("600000")
	efft.Effect(into[int64]("-1K")).Equals("-1000")
	efft.Effect(into[int64]("10Ki")).Equals("10240")
	efft.Effect(into[int64]("10mG")).Equals("sdin.UnsupportedNumberUnit unit=mG token=\"10mG\"")
	efft.Effect(into[int64]("10m1s")).Equals("sdin.ParseInt: strconv.ParseInt: parsing \"10m1\": invalid syntax")
	efft.Effect(into[int64]("1.123s")).Equals("1123")
	efft.Effect(into[int64]("1e9")).Equals("1000000000")
	efft.Effect(into[int64]("1.1234s")).Equals("sdin.FractionalFloatAsInteger s=1.1234s: floats with nonzero fractional part cannot be converted to an integer without loss")
	efft.Effect(into[int64]("1e-3s")).Equals("1")
	efft.Effect(into[int64]("1e-12s")).Equals("sdin.FractionalFloatAsInteger s=1e-12s: floats with nonzero fractional part cannot be converted to an integer without loss")
	efft.Effect(into[int64]("123_456")).Equals("123456")
	efft.Effect(into[int64]("123_456_K")).Equals("123456000")
	efft.Effect(into[int64]("123_456P")).Equals("sdin.NumTooLarge v=123_456P")
	efft.Effect(into[int64]("9223372036854775808")).Equals("sdin.LargeFloatAsInteger s=9223372036854775808: this number is outside the safe integer range of floats (-2^53, 2^53)")
	efft.Effect(into[int64]("1e20")).Equals("sdin.LargeFloatAsInteger s=1e20: this number is outside the safe integer range of floats (-2^53, 2^53)")

	efft.Effect(into[int]("123")).Equals("123")
	efft.Effect(into[int8]("123")).Equals("123")
	efft.Effect(into[int8]("-129")).Equals("sdin.IntOutOfRange s=-129 type=int8")
	efft.Effect(into[int8]("+127")).Equals("127")
	efft.Effect(into[int8]("1024")).Equals("sdin.IntOutOfRange s=1024 type=int8")
	efft.Effect(into[int16]("123")).Equals("123")
	efft.Effect(into[int32]("123")).Equals("123")
	efft.Effect(into[int64]("123")).Equals("123")
	efft.Effect(into[int64]("+0x10")).Equals("sdin.ParseInt: strconv.ParseInt: parsing \"+0x10\": invalid syntax")
	efft.Effect(into[uint]("123")).Equals("123")
	efft.Effect(into[uint8]("123")).Equals("123")
	efft.Effect(into[uint8]("256")).Equals("sdin.UintOutOfRange s=256 type=uint8")
	efft.Effect(into[uint16]("-1")).Equals("sdin.NotANumberToken value=\"-1\"")
	efft.Effect(into[uint16]("123")).Equals("123")
	efft.Effect(into[uint32]("123")).Equals("123")
	efft.Effect(into[uint64]("123")).Equals("123")
	efft.Effect(into[uint64]("0xffff_ffff_ffff_ffff")).Equals("18446744073709551615")
	efft.Effect(into[uint64]("+0x10")).Equals("sdin.NotANumberToken value=\"+0x10\"")
	efft.Effect(into[float32]("123")).Equals("123")
	efft.Effect(into[float32]("1e100")).Equals("sdin.ParseFloat: strconv.ParseFloat: parsing \"1e100\": value out of range")
	efft.Effect(into[float32]("1e30P")).Equals("sdin.Float32Overflow s=1e30P")
	efft.Effect(into[float64]("123")).Equals("123")
	efft.Effect(into[int64]("+")).Equals("sdin.ParseInt: strconv.ParseInt: parsing \"+\": invalid syntax")
	efft.Effect(into[uint64]("+")).Equals("sdin.NotANumberToken value=\"+\"")
	efft.Effect(into[float64]("+")).Equals("sdin.ParseFloat: strconv.ParseFloat: parsing \"+\": invalid syntax")

	efft.Effect(into[time.Duration]("2s")).Equals("2s")
	efft.Effect(into[time.Duration]("1.5s")).Equals("1.5s")
	efft.Effect(into[time.Duration]("1.5h")).Equals("1h30m0s")
	efft.Effect(into[time.Duration]("1.001s")).Equals("1.001s")
	efft.Effect(into[time.Duration]("1.1234s")).Equals("1.1234s")
	efft.Effect(into[time.Duration]("0.5w")).Equals("84h0m0s")
	efft.Effect(into[time.Duration]("1Mi")).Equals("sdin.NotADuration s=\"1Mi\" (duration must have a s,m,h,d,w unit)")
	efft.Effect(into[time.Duration]("1m2s")).Equals("sdin.ParseFloat: strconv.ParseFloat: parsing \"1m2\": invalid syntax")
	efft.Effect(into[time.Duration]("1.5")).Equals("sdin.NotADuration s=\"1.5\" (duration must have a s,m,h,d,w unit)")
	efft.Effect(into[int]("1.5s")).Equals("1500")
	efft.Effect(into[float64]("1.5s")).Equals("1500")
	efft.Effect(into[int]("1.51234s")).Equals("sdin.FractionalFloatAsInteger s=1.51234s: floats with nonzero fractional part cannot be converted to an integer without loss")
	efft.Effect(into[float64]("1.51234s")).Equals("1512.34")

	defer func(orig *time.Location) { time.Local = orig }(time.Local)
	time.Local = efft.Must1(time.LoadLocation("Europe/Paris"))
	efft.Effect(into[int64]("1970-01-01@UTC")).Equals("0")
	efft.Effect(into[int64]("1970-01-01@Asia/Tokyo")).Equals("-32400000")
	efft.Effect(into[int64]("1970-01-01@local")).Equals("-3600000")
	efft.Effect(into[int64]("1970-01-01")).Equals("0")
	efft.Effect(into[int64]("1970-01-02@America/Nowhere")).Equals("sdin.LoadTimezone value=\"1970-01-02@America/Nowhere\": unknown time zone America/Nowhere")
	efft.Effect(into[int64]("1970-01-02.000101")).Equals("86461000")
	efft.Effect(into[int64]("1970-01-02.000101.123")).Equals("86461123")
	efft.Effect(into[float64]("1970-01-02.000101.123")).Equals("8.6461123e+07")
	efft.Effect(into[int64]("1970-01-02.000101.123456")).Equals("86461123")
	efft.Effect(into[float64]("1970-01-02.000101.123456")).Equals("8.6461123456e+07")
	efft.Effect(into[float64]("1970-01-02")).Equals("8.64e+07")
	efft.Effect(into[float64]("1970-01-02.000101")).Equals("8.6461e+07")

	efft.Effect(into[time.Time]("1970-01-01@UTC")).Equals("1970-01-01 00:00:00 +0000 UTC")
	efft.Effect(into[time.Time]("1970-01-01.01:02:03@Asia/Tokyo")).Equals("sdin.UnsupportedTimeFormat value=\"1970-01-01.01:02:03@Asia/Tokyo\"")
	efft.Effect(into[time.Time]("1970-01-01.01_02_03.456789@Asia/Tokyo")).Equals("1970-01-01 01:02:03.456789 +0900 JST")
	efft.Effect(into[time.Time]("1234_56_78")).Equals("sdin.NotADateTime s=\"1234_56_78\"")

	efft.Effect(into[tokens.JSONValue]("123.456")).Equals("\"123.456\"")
	efft.Effect(into[tokens.JSONValue]("+123.456")).Equals("\"123.456\"")
	efft.Effect(into[tokens.JSONValue]("-1e-9")).Equals("\"-1e-9\"")
	efft.Effect(into[tokens.JSONValue]("2Ki")).Equals("\"2048\"")
	efft.Effect(into[tokens.JSONValue]("1_048_576")).Equals("\"1048576\"")
	efft.Effect(into[tokens.JSONValue]("1970-01-02")).Equals("\"86400000\"")
	efft.Effect(into[tokens.JSONValue]("inf")).Equals("sdin.ParseJSONFloat: sdin.NotANumberToken value=\"inf\"")
	efft.Effect(into[tokens.JSONValue]("1e-12s")).Equals("\"0.000000\"")
	efft.Effect(into[tokens.JSONValue]("0.5")).Equals("\"0.5\"")
	efft.Effect(into[tokens.JSONValue](".5")).Equals("sdin.ParseJSONFloat: sdin.NotANumberToken value=\".5\"")
	efft.Effect(into[tokens.JSONValue]("-.5")).Equals("\"-0.500000\"")
	efft.Effect(into[tokens.JSONValue]("+.5")).Equals("\"0.500000\"")
	efft.Effect(into[tokens.JSONValue]("5.")).Equals("\"5\"")
	efft.Effect(into[tokens.JSONValue]("hello")).Equals("sdin.ParseJSONFloat: sdin.NotANumberToken value=\"hello\"")
	efft.Effect(into[tokens.JSONValue]("'hello \" quote'")).Equals("\"\\\"hello \\\\\\\" quote\\\"\"")
	efft.Effect(into[tokens.JSONValue]("false")).Equals("\"false\"")

	efft.Effect(into[tag]("foo")).Equals("sdin.FromSDINToken type=*sdin_test.tag: sdintest.TagWithoutSeparator token=\"foo\"")
	efft.Effect(into[tag]("hello=world")).Equals(`
		{
		  "Key": "hello",
		  "Value": "world"
		}`)

	efft.Note = "from custom type"
	type color string
	efft.Effect(into[color](`"red"`)).Equals("\"red\"")
	type blob []byte
	efft.Effect(string(efft.Must1(into[blob](`"blobtext"`)))).Equals("blobtext")
}

func TestMakeToken(t *testing.T) {
	efft.Init(t)
	from := func(v any) string { return sdin.MakeToken(v).Repr() }

	efft.Note = "from nil"
	efft.Effect(from(nil)).Equals("nil")

	efft.Note = "from Token"
	efft.Effect(from(sdin.MakeTokenFromRepr("hello"))).Equals("hello")
	efft.Effect(from(sdin.MakeTokenFromRepr("'hello'"))).Equals("'hello'")

	efft.Note = "from bool"
	efft.Effect(from(true)).Equals("true")
	efft.Effect(from(false)).Equals("false")

	efft.Note = "from intoToken"
	var panicstr string
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicstr = r.(string)
			}
		}()
		from(tag{"hello", "world"})
	}()
	efft.Effect(panicstr).Equals("sdin.UnsupportedTokenFromNonPointer type=sdin_test.tag: pass a pointer, that one implements IntoSDINToken")
	efft.Effect(from(&tag{"key", "value"})).Equals("key=value")

	efft.Note = "from identifier"
	efft.Effect(from(sdin.Identifier("HelloWorld"))).Equals("HelloWorld")

	efft.Note = "from []byte"
	efft.Effect(from([]byte(`hello world`))).Equals("'hello world'")
	efft.Effect(from([]byte(`hello"world`))).Equals("'hello\"world'")
	efft.Effect(from([]byte(`hello'world`))).Equals("\"hello'world\"")
	efft.Effect(from([]byte(`hello'"world`))).Equals("\"hello'\\\"world\"")

	efft.Note = "from string"
	efft.Effect(from(`hello world`)).Equals("'hello world'")
	efft.Effect(from(`hello"world`)).Equals("'hello\"world'")
	efft.Effect(from(`hello'world`)).Equals("\"hello'world\"")
	efft.Effect(from(`hello'"world`)).Equals("\"hello'\\\"world\"")

	efft.Note = "from unquoted StringToken"
	efft.Effect(from(sdin.StringToken{Data: `hello world`})).Equals("'hello world'")
	efft.Effect(from(sdin.StringToken{Data: `hello"world`})).Equals("'hello\"world'")
	efft.Effect(from(sdin.StringToken{Data: `hello'world`})).Equals("'''|hello'world'''")
	efft.Effect(from(sdin.StringToken{Data: `hello'"world`})).Equals("'''|hello'\"world'''")
	efft.Effect(from(sdin.StringToken{Data: `hello''world`})).Equals("'''|hello''world'''")
	efft.Effect(from(sdin.StringToken{Data: `hello'''world`})).Equals("''''|hello'''world''''")
	efft.Effect(from(sdin.StringToken{Data: `hello''''world`})).Equals("'''''|hello''''world'''''")

	efft.Note = "from StringToken"
	efft.Effect(from(sdin.StringToken{Quote: `"`, Data: `hello"world`})).Equals("\"hello\\\"world\"")
	efft.Effect(from(sdin.StringToken{Quote: `'`, Data: `hello"world`})).Equals("'hello\"world'")
	efft.Effect(from(sdin.StringToken{Quote: "'|", Data: `hello world`})).Equals("'''|hello world'''")
	efft.Effect(from(sdin.StringToken{Quote: "'|", Data: "hello'''world"})).Equals("''''|hello'''world''''")
	efft.Effect(from(sdin.StringToken{Quote: "'some tag|", Data: "hello`world"})).Equals("'''some tag|hello`world'''")
	efft.Effect(from(sdin.StringToken{Quote: "'some tag\n", Data: "hello\nworld"})).Equals("'''some tag|hello\nworld'''")
	efft.Effect(from(sdin.StringToken{Quote: "'some tag\n", Data: "hello\nworld\n"})).Equals("'''some tag\nhello\nworld\n'''")
	efft.Effect(from(sdin.StringToken{Quote: "'some tag\n\t", Data: "hello\nworld\n"})).Equals("'''some tag\n\thello\n\tworld\n\t'''")
	efft.Effect(from(sdin.StringToken{Quote: "'some tag\n\t", Data: "hello\n\n\nworld\n"})).Equals("'''some tag\n\thello\n\n\n\tworld\n\t'''")
	efft.Effect(from(sdin.StringToken{Quote: "'some'''tag|", Data: "hello world"})).Equals("''''some'''tag|hello world''''")

	efft.Note = "from float64"
	efft.Effect(from(42.0)).Equals("42")
	efft.Effect(from(42.00000001)).Equals("42.00000001")
	efft.Effect(from(42e9)).Equals("4.2e+10")
	efft.Effect(from(42e-9)).Equals("4.2e-08")
	efft.Effect(from(1 - 1e-9)).Equals("0.999999999")
	efft.Effect(from(1 - 1e-99)).Equals("1")

	efft.Note = "from int64"
	efft.Effect(from(0)).Equals("0")
	efft.Effect(from(1)).Equals("1")
	efft.Effect(from(-11)).Equals("-11")
	efft.Effect(from(1000)).Equals("1000")
	efft.Effect(from(-1000)).Equals("-1000")
	efft.Effect(from(1024)).Equals("1024")
	efft.Effect(from(-1024)).Equals("-1024")
	efft.Effect(from(1001)).Equals("1001")
	efft.Effect(from(-1001)).Equals("-1001")
	efft.Effect(from(1025)).Equals("1025")
	efft.Effect(from(-1025)).Equals("-1025")
	efft.Effect(from(34_000_000)).Equals("34M")
	efft.Effect(from(35_651_584)).Equals("34Mi")
	efft.Effect(from(3600_000)).Equals("3600000")

	efft.Note = "from sized types"
	efft.Effect(from(int(123))).Equals("123")
	efft.Effect(from(int8(123))).Equals("123")
	efft.Effect(from(int16(123))).Equals("123")
	efft.Effect(from(int32(123))).Equals("123")
	efft.Effect(from(int64(123))).Equals("123")
	efft.Effect(from(int64(-1 << 63))).Equals("-8192Pi")
	efft.Effect(from(uint(123))).Equals("123")
	efft.Effect(from(uint8(123))).Equals("123")
	efft.Effect(from(uint16(123))).Equals("123")
	efft.Effect(from(uint32(123))).Equals("123")
	efft.Effect(from(uint64(123))).Equals("123")
	efft.Effect(from(uint64(1<<64 - 1))).Equals("18446744073709551615")
	efft.Effect(from(float32(0.1))).Equals("0.1")
	efft.Effect(from(float64(0.1))).Equals("0.1")
	efft.Effect(from(float32(123))).Equals("123")
	efft.Effect(from(float64(123))).Equals("123")

	efft.Note = "from time.Duration"
	efft.Effect(from(0 * time.Hour)).Equals("0s")
	efft.Effect(from(4 * time.Hour)).Equals("4h")
	efft.Effect(from(2*time.Hour + 2*time.Minute)).Equals("122m")
	efft.Effect(from(8 * 24 * time.Hour)).Equals("8d")
	efft.Effect(from(1 * time.Second)).Equals("1s")
	efft.Effect(from(1*time.Second + 2*time.Millisecond)).Equals("1.002s")
	efft.Effect(from(2 * time.Millisecond)).Equals("0.002s")
	efft.Effect(from(1*time.Second + 2*time.Microsecond)).Equals("1.000002s")

	efft.Note = "from time.Time"
	defer func(orig *time.Location) { time.Local = orig }(time.Local)
	time.Local = efft.Must1(time.LoadLocation("Europe/Paris"))
	efft.Effect(from(time.Time{})).Equals("0001-01-01")
	efft.Effect(from(time.Unix(1, 0))).Equals("1970-01-01.010001@local")
	efft.Effect(from(time.UnixMilli(1))).Equals("1970-01-01.010000.001@local")
	efft.Effect(from(time.UnixMicro(1))).Equals("1970-01-01.010000.000001@local")
	efft.Effect(from(time.Unix(0, 1))).Equals("1970-01-01.010000.000000001@local")
	efft.Effect(from(time.Unix(1, 1))).Equals("1970-01-01.010001.000000001@local")
	efft.Effect(from(time.Date(2001, 01, 02, 03, 04, 05, 06, time.UTC))).Equals("2001-01-02.030405.000000006")
	efft.Effect(from(time.Date(2001, 01, 02, 03, 04, 05, 06, time.Local))).Equals("2001-01-02.030405.000000006@local")
	efft.Effect(from(time.Date(2001, 01, 02, 03, 04, 05, 06, time.FixedZone("fixed", 13)))).Equals("2001-01-02.030405.000000006@fixed")
	efft.Effect(from(time.Date(2001, 01, 02, 03, 04, 05, 06, efft.Must1(time.LoadLocation("Asia/Tokyo"))))).Equals("2001-01-02.030405.000000006@Asia/Tokyo")
	efft.Effect(from(time.Date(2001, 01, 02, 03, 04, 05, 00, time.UTC))).Equals("2001-01-02.030405")
	efft.Effect(from(time.Date(2001, 01, 02, 03, 04, 00, 00, time.UTC))).Equals("2001-01-02.0304")
	efft.Effect(from(time.Date(2001, 01, 02, 03, 00, 00, 00, time.UTC))).Equals("2001-01-02.0300")
	efft.Effect(from(time.Date(2001, 01, 02, 00, 00, 00, 00, time.UTC))).Equals("2001-01-02")
	efft.Effect(from(time.Date(2001, 01, 00, 00, 00, 00, 00, time.UTC))).Equals("2000-12-31")
	efft.Effect(from(efft.Must1(time.Parse(time.RFC3339, "2001-01-02T15:04:05+02:00")))).Equals("2001-01-02.130405")
	efft.Effect(from(efft.Must1(time.Parse(time.RFC3339, "2001-01-02T15:04:05+00:00")))).Equals("2001-01-02.150405")

	efft.Note = "from custom type"
	type color string
	var c color = "red"
	efft.Effect(from(c)).Equals("'red'")
	type blob []byte
	var b blob = []byte("blobtext")
	efft.Effect(from(b)).Equals("'blobtext'")

	efft.Note = "e2e test: parse, rewrite AST, format"
	n := efft.Must1(sdin.Parse(sdin.TFlatStruct(), `Hello: "Alice"`))
	n.Children[0].Name = "Hi"
	n.Children[0].Value = sdin.MakeToken(sdin.StringToken{Quote: "'|", Data: "Bob"})
	efft.Effect(n.Format()).Equals("Hi: '''|Bob'''")
}
