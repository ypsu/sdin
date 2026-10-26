package sdin_test

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ypsu/efftesting/efft"
	"github.com/ypsu/sdin"
)

type pair struct{ Key, Value string }

func (t *pair) FromSDINToken(token sdin.Token) error {
	k, v, ok := strings.Cut(token.Repr(), "=")
	if !ok {
		return fmt.Errorf("sdintest.TagWithoutSeparator token=%q", token.Repr())
	}
	t.Key, t.Value = k, v
	return nil
}

func (t *pair) IntoSDINToken() sdin.Token {
	return sdin.MakeTokenFromRepr(t.Key + "=" + t.Value)
}

type pairmap struct{ m map[string]string }

func (m *pairmap) FromSDINNode(n *sdin.Node) error {
	if !n.Kind().Struct() {
		return fmt.Errorf("sdintest.ParseNonObjectPairmap")
	}
	if m.m == nil {
		m.m = make(map[string]string, len(n.Children))
	}
	for _, ch := range n.Children {
		v, err := ch.Value.As[string]()
		if err != nil {
			return fmt.Errorf("sdintest.ParsePairmapString: %v", err)
		}
		m.m[ch.Name] = v
	}
	return nil
}

func (m *pairmap) IntoSDINNode() *sdin.Node {
	n := sdin.NewNode("", sdin.MakeTokenFromRepr("{"))
	for _, k := range slices.Sorted(maps.Keys(m.m)) {
		n.Children = append(n.Children, sdin.NewNode(k, m.m[k]))
	}
	return n
}

type idnum int

func (id *idnum) FromSDINToken(t sdin.Token) error {
	s := strings.TrimPrefix(t.Repr(), "id")
	v, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("sdintest.ParseIDNum token=%q: %v", t, err)
	}
	*id = idnum(v)
	return nil
}

type commit struct {
	Author      sdin.Identifier
	Date        time.Time
	Description string
	Files       []string
	Tags        map[sdin.Identifier]string
}

func TestNodeAs(t *testing.T) {
	efft.Init(t)

	n := efft.Must1(sdin.Parse(sdin.TFlatStruct(), `Year:1999 Month:"January" Day:30 weekday:{name:"Saturday"} Names:["alice" "bob"]`))
	type weekday struct{ Name string }
	type date struct {
		Year    int
		Month   string
		Day     int
		Weekday weekday
		Names   []string
	}
	efft.Effect(n.As[date]()).Equals(`
		{
		  "Year": 1999,
		  "Month": "January",
		  "Day": 30,
		  "Weekday": {
		    "Name": "Saturday"
		  },
		  "Names": [
		    "alice",
		    "bob"
		  ]
		}`)

	type partialDate struct {
		Year                      int
		Day                       int
		_discardUnknownSDINFields struct{}
	}
	n = efft.Must1(sdin.Parse(sdin.TFlatStruct(), `Year:1999 Month:"January" Day:30 weekday:{name:"Saturday"} Names:["alice" "bob"]`))
	efft.Effect(n.As[partialDate]()).Equals(`
		{
		  "Year": 1999,
		  "Day": 30
		}`)

	n = efft.Must1(sdin.Parse(sdin.TFlatStruct(), `alice:1 bob:2 charlie:4`))
	efft.Effect(n.As[map[sdin.Identifier]int]()).Equals(`
		{
		  "alice": 1,
		  "bob": 2,
		  "charlie": 4
		}`)

	n = efft.Must1(sdin.Parse(sdin.TFlatStruct(), `somemap:{hello:[{Year:1} {Month:"Feb"} {Names:["alice" "bob"]}]}`))
	type someMapStruct struct{ SomeMap map[string][]date }
	efft.Effect(n.As[someMapStruct]()).Equals(`
		{
		  "SomeMap": {
		    "hello": [
		      {
		        "Year": 1,
		        "Month": "",
		        "Day": 0,
		        "Weekday": {
		          "Name": ""
		        },
		        "Names": null
		      },
		      {
		        "Year": 0,
		        "Month": "Feb",
		        "Day": 0,
		        "Weekday": {
		          "Name": ""
		        },
		        "Names": null
		      },
		      {
		        "Year": 0,
		        "Month": "",
		        "Day": 0,
		        "Weekday": {
		          "Name": ""
		        },
		        "Names": [
		          "alice",
		          "bob"
		        ]
		      }
		    ]
		  }
		}`)

	type ptrstruct struct {
		Int     int
		IntPtr  *int
		Tag     pair
		TagPtr  *pair
		TagPtr2 *pair
	}
	n = efft.Must1(sdin.Parse(sdin.TFlatStruct(), `
		Int:    3
		IntPtr: 4
		Tag:    "hello=world"
		TagPtr: 'world=hello'
	`))
	efft.Effect(n.As[ptrstruct]()).Equals(`
		{
		  "Int": 3,
		  "IntPtr": 4,
		  "Tag": {
		    "Key": "\"hello",
		    "Value": "world\""
		  },
		  "TagPtr": {
		    "Key": "'world",
		    "Value": "hello'"
		  },
		  "TagPtr2": null
		}`)

	n = efft.Must1(sdin.Parse(sdin.TFlatStruct(), `
		alice: "Alice Alicious"
		bob:   "Robert Roberticious"
	`))
	efft.Effect(efft.Must1(n.As[pairmap]()).m).Equals(`
		{
		  "alice": "Alice Alicious",
		  "bob": "Robert Roberticious"
		}`)
	efft.Effect(efft.Must1(sdin.Parse(sdin.TFlatStruct(), `x:[]`)).Get("x").As[pairmap]()).Equals("sdin.FromSDINNode line=1: sdintest.ParseNonObjectPairmap")

	n = efft.Must1(sdin.Parse(sdin.TFlatStruct(), `
		id123: OneTwoThree
		id234: TwoThreeFour
	`))
	efft.Effect(n.As[map[idnum]sdin.Identifier]()).Equals(`
		{
		  "123": "OneTwoThree",
		  "234": "TwoThreeFour"
		}`)

	n = efft.Must1(sdin.Parse(sdin.TFlatStruct(), `
		SomeField: 4
		Object: {
			Field1: 1
			Field2: ["hello"]
		}`))
	object := efft.Must1(n.As[struct {
		SomeField int
		Object    sdin.Node
	}]())
	efft.Effect(object.SomeField).Equals("4")
	efft.Effect(object.Object.Format()).Equals(`
		{
			Field1: 1
			Field2: ["hello"]
		}
		`)
	object2 := efft.Must1(n.As[struct {
		SomeField int
		Object    *sdin.Node
	}]())
	efft.Effect(object2.SomeField).Equals("4")
	efft.Effect(object2.Object.Format()).Equals(`
		{
			Field1: 1
			Field2: ["hello"]
		}
		`)

	n = efft.Must1(sdin.Parse(sdin.TFlatStruct(), `Key:func(7)`))
	type keyint struct{ Key int }
	efft.Effect(n.As[keyint]()).Equals("sdin.UnresolvedForm line=1 path=Key name=\"func(\"")

	n = efft.Must1(sdin.Parse(sdin.TFlatStruct(), `
		Author:      alice
		Date:        2006-01-02.030405
		Description: '''
			Implement the frobnication

			Do so via barificating the bazificator.
			Unit tested.
			'''
		Files: [
			'bar.go'
			'frob.go'
		]
		Tags: {
			reviewerer: 'bob,charlie'
			risk:       'none'
			tested:     'true'
		}
	`))
	efft.Effect(n.As[commit]()).Equals(`
		{
		  "Author": "alice",
		  "Date": "2006-01-02T03:04:05Z",
		  "Description": "Implement the frobnication\n\nDo so via barificating the bazificator.\nUnit tested.\n",
		  "Files": [
		    "bar.go",
		    "frob.go"
		  ],
		  "Tags": {
		    "reviewerer": "bob,charlie",
		    "risk": "none",
		    "tested": "true"
		  }
		}`)
}

func TestNewNode(t *testing.T) {
	efft.Init(t)

	efft.Note = "from IntoSDINToken"
	var panicstr string
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicstr = r.(string)
			}
		}()
		sdin.NewNode("", pair{"hello", "world"})
	}()
	efft.Effect(panicstr).Equals("sdin.PointerWouldImplementIntoSDINToken type=sdin_test.pair: pass a pointer to NewNode")
	efft.Effect(sdin.NewNode("", &pair{"hello", "world"})).Equals("hello=world")

	efft.Note = "from IntoSDINNode"
	panicstr = ""
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicstr = r.(string)
			}
		}()
		sdin.NewNode("", pairmap{m: map[string]string{"k1": "v1", "k2": "v2"}})
	}()
	efft.Effect(panicstr).Equals("sdin.PointerWouldImplementIntoSDINNode type=sdin_test.pairmap: pass a pointer to NewNode")
	efft.Effect(sdin.NewNode("", &pairmap{m: map[string]string{"k1": "v1", "k2": "v2"}})).Equals("{k1:\"v1\" k2:\"v2\"}")

	efft.Note = "from slice"
	efft.Effect(sdin.NewNode("", []int{1, 2, 3})).Equals("[1 2 3]")
	efft.Effect(sdin.NewNode("", []string{"hello", "world"})).Equals("[\"hello\" \"world\"]")
	efft.Effect(sdin.NewNode("", []*pair{{"hello1", "world1"}})).Equals("[hello1=world1]")
	efft.Effect(sdin.NewNode("", []pair{{"hello2", "world2"}})).Equals("[hello2=world2]")
	efft.Effect(sdin.NewNode("", []pairmap{{m: map[string]string{"k1": "v1", "k2": "v2"}}})).Equals("[{k1:\"v1\" k2:\"v2\"}]")
	efft.Effect(sdin.NewNode("", []*pairmap{{m: map[string]string{"k1": "v1", "k2": "v2"}}})).Equals("[{k1:\"v1\" k2:\"v2\"}]")

	efft.Note = "from struct"
	type astruct struct {
		Int     int
		Str     string
		ID      sdin.Identifier
		PtrInt  *int
		Pair    pair
		PtrPair *pair
		Ints    []int
		A       *astruct
	}
	sv := astruct{Int: 2}
	efft.Effect(sdin.NewNode("", sv).Format()).Equals(`
		{
			Int: 2
		}
		`,
	)
	efft.Effect(sdin.NewFlatNode("", sv).Format()).Equals(`
		Int: 2
		`,
	)
	x := 0
	svp := &astruct{Int: 0, Str: "hello", ID: "world", PtrInt: &x, Pair: pair{"k1", "v1"}, PtrPair: &pair{"k2", "v2"}, Ints: []int{6}}
	efft.Effect(sdin.NewFlatNode("", svp).Format()).Equals(`
		Str:     'hello'
		ID:      world
		PtrInt:  0
		Pair:    k1=v1
		PtrPair: k2=v2
		Ints: [
			6
		]
		`,
	)

	efft.Note = "from map"
	efft.Effect(sdin.NewNode("", map[int]int{1: 2, 2: 4, 3: 6}).Format()).Equals(`
		map(
			1 2
			2 4
			3 6
		)
		`)
	efft.Effect(sdin.NewNode("", map[int][]int{1: {2}, 2: {4, 5}, 3: {}}).Format()).Equals(`
		map(
			1 [
				2
			]
			2 [
				4
				5
			]
			3 []
		)
		`,
	)
	n := sdin.NewNode("", map[any]any{1: []string(nil), 2: []int{6}, sdin.Identifier("hello"): &pair{"k", "v"}})
	efft.Effect(n.Format()).Equals(`
		map(
			1 []
			2 [
				6
			]
			hello k=v
		)
		`,
	)

	efft.Note = "from Node"
	efft.Effect(sdin.NewFlatNode("", n).Format()).Equals(`
		map(
			1 []
			2 [
				6
			]
			hello k=v
		)
		`,
	)

	efft.Note = "from complex struct"
	cv := &commit{
		Author:      "alice",
		Date:        time.Date(2006, 1, 2, 3, 4, 5, 0, time.UTC),
		Description: "Implement the frobnication\n\nDo so via barificating the bazificator.\nUnit tested.\n",
		Files:       []string{"bar.go", "frob.go"},
		Tags: map[sdin.Identifier]string{
			"tested":   "true",
			"reviewer": "bob,charlie",
			"risk":     "none",
		},
	}
	n = sdin.NewFlatNode("", cv)
	efft.Effect(n.Format()).Equals(`
		Author:      alice
		Date:        2006-01-02.030405
		Description: "Implement the frobnication\n\nDo so via barificating the bazificator.\nUnit tested.\n"
		Files: [
			'bar.go'
			'frob.go'
		]
		Tags: map(
			reviewer 'bob,charlie'
			risk     'none'
			tested   'true'
		)
		`,
	)
}

func TestMarshaling(t *testing.T) {
	efft.Init(t)

	// Custom string map.
	type id string
	efft.Effect(sdin.Unmarshal[map[id]int](`ID1:1 ID2:2`)).Equals(`
		{
		  "ID1": 1,
		  "ID2": 2
		}`)

	p := efft.Must1(sdin.Unmarshal[struct{ Pair pair }]("Pair:hello=world"))
	efft.Effect(p).Equals(`
		{
		  "Pair": {
		    "Key": "hello",
		    "Value": "world"
		  }
		}`)
	efft.Effect(sdin.Marshal(&p)).Equals(`
		Pair: hello=world
		`,
	)

	ps := efft.Must1(sdin.Unmarshal[[]pair]("{Key:'hello1' Value:'world1'} hello2=world2"))
	efft.Effect(ps).Equals(`
		[
		  {
		    "Key": "hello1",
		    "Value": "world1"
		  },
		  {
		    "Key": "hello2",
		    "Value": "world2"
		  }
		]`)
	efft.Effect(sdin.Marshal(ps)).Equals(`
		hello1=world1
		hello2=world2
		`,
	)

	badkeys := sdin.Node{Value: sdin.TFlatStruct()}
	badkeys.Children = append(badkeys.Children, sdin.NewNode("", "MissingKey"))
	badkeys.Children = append(badkeys.Children, sdin.NewNode("'tükör-gép'", "BadKeyCharacters"))
	efft.Effect(sdin.Marshal(badkeys)).Equals(`
		__MissingNameError__: 'MissingKey'
		_tükör_gép_:          'BadKeyCharacters'
		`)

	m := map[string]int{"two": 2}
	efft.Effect(sdin.Marshal(m)).Equals(`
		map(
			'two' 2
		)
		`)
}

func TestCloning(t *testing.T) {
	efft.Init(t)

	n := efft.Must1(sdin.Unmarshal[*sdin.Node](`Struct1: {
			IntValue:  42
			ListValue: [1 2 3]
		}
	`))

	n.Children = append(n.Children, sdin.NewNode("Struct2", n.Children[0]))
	n.Children[0].Name = "Struct3"
	n.Children[0].Children[0].Value = sdin.MakeToken(43)
	n.Children[0].Children[1].Children[2].Value = sdin.MakeToken(5)

	efft.Effect(n.Format()).Equals(`
		Struct3: {
			IntValue:  43
			ListValue: [1 2 5]
		}
		Struct2: {
			IntValue:  42
			ListValue: [1 2 3]
		}
		`)
}

type NamedInt int

func (i NamedInt) IntoSDINNode() *sdin.Node {
	return sdin.NewNode("V"+strconv.Itoa(int(i)), int(i))
}

func TestNewNodeName(t *testing.T) {
	efft.Init(t)
	n := sdin.NewFlatNode("", sdin.TFlatForm())
	n.Children = append(n.Children, NamedInt(3).IntoSDINNode())
	n.Children = append(n.Children, sdin.NewNode("", NamedInt(4)))
	n.Children = append(n.Children, sdin.NewNode("Five", NamedInt(5)))
	n.Children = append(n.Children, sdin.NewNode("Six", struct{ Value NamedInt }{6}))
	efft.Effect(n).Equals("V3:3 4 Five:5 Six:{Value:6}")
}

func TestListParsing(t *testing.T) {
	efft.Init(t)

	efft.Effect(sdin.Unmarshal[[]string](`hello world`)).Equals("sdin.IntoForUnmarshal: sdin.TokenInto line=1 path=0: sdin.ParseStringToken: sdin.ParseBareAsString s=\"hello\": bare tokens are not considered as strings")
	efft.Effect(sdin.Unmarshal[[]string](`hello-world`)).Equals("sdin.IntoForUnmarshal: sdin.TokenInto line=1 path=0: sdin.ParseStringToken: sdin.ParseBareAsString s=\"hello-world\": bare tokens are not considered as strings")
	efft.Effect(sdin.Unmarshal[[]string](`'hello' 'world'`)).Equals(`
		[
		  "hello",
		  "world"
		]`)

	efft.Effect(sdin.Unmarshal[[]sdin.Identifier](`hello world`)).Equals(`
		[
		  "hello",
		  "world"
		]`)
	efft.Effect(sdin.Unmarshal[[]sdin.Identifier](`hello-world`)).Equals("sdin.IntoForUnmarshal: sdin.TokenInto line=1 path=0: sdin.NonIdentifierCharacter i=5 ch='-' s=\"hello-world\": identifier must contain only letters, numbers, or underscores")
	efft.Effect(sdin.Unmarshal[[]sdin.Identifier](`'hello' 'world'`)).Equals("sdin.IntoForUnmarshal: sdin.TokenInto line=1 path=0: sdin.StringAsIdentifier s=\"'hello'\": strings are not considered identifiers")

	efft.Effect(sdin.Unmarshal[[]sdin.BareToken](`hello world`)).Equals(`
		[
		  "hello",
		  "world"
		]`)
	efft.Effect(sdin.Unmarshal[[]sdin.BareToken](`hello-world`)).Equals(`
		[
		  "hello-world"
		]`)
	efft.Effect(sdin.Unmarshal[[]sdin.BareToken](`[x]`)).Equals("sdin.IntoForUnmarshal: sdin.UnsupportedIntoKind line=1 path=0 kind=string: check sdin.Interface for adding support")
	efft.Effect(sdin.Unmarshal[[]sdin.BareToken](`'hello' 'world'`)).Equals("sdin.IntoForUnmarshal: sdin.TokenInto line=1 path=0: sdin.StringAsBareToken")

	var tokens []string
	for _, t := range efft.Must1(sdin.Unmarshal[[]sdin.Token](`hello 'world'`)) {
		tokens = append(tokens, t.Repr())
	}
	efft.Effect(tokens).Equals(`
		[
		  "hello",
		  "'world'"
		]`)
}

func TestNils(t *testing.T) {
	efft.Init(t)

	const sdinData = `
		PresentInt1:    0
		PresentInt2:    0
		PresentList:    []
		PresentMap:     map()
		PresentStruct1: {}
		PresentStruct2: {}
		`

	type astruct struct{ Num int }

	type data struct {
		PresentInt1    int
		NotPresentInt1 int

		PresentInt2    *int
		NotPresentInt2 *int

		PresentList    []int
		NotPresentList []int

		PresentMap    map[int]int
		NotPresentMap map[int]int

		PresentStruct1    astruct
		NotPresentStruct1 astruct

		PresentStruct2    *astruct
		NotPresentStruct2 *astruct
	}

	d := efft.Must1(sdin.Unmarshal[data](sdinData))
	efft.Effect(d).Equals(`
		{
		  "PresentInt1": 0,
		  "NotPresentInt1": 0,
		  "PresentInt2": 0,
		  "NotPresentInt2": null,
		  "PresentList": [],
		  "NotPresentList": null,
		  "PresentMap": {},
		  "NotPresentMap": null,
		  "PresentStruct1": {
		    "Num": 0
		  },
		  "NotPresentStruct1": {
		    "Num": 0
		  },
		  "PresentStruct2": {
		    "Num": 0
		  },
		  "NotPresentStruct2": null
		}`,
	)
	efft.Effect(sdin.Marshal(d)).Equals(`
		PresentInt2:    0
		PresentList:    []
		PresentMap:     map()
		PresentStruct2: {}
		`,
	)
}

func TestErrPath(t *testing.T) {
	efft.Init(t)

	const sdinData = `
		OutermostStruct: {
			InnerList: [
				a=b
				c?d
			]
		}
	`
	type data struct {
		OutermostStruct struct {
			InnerList []pair
		}
	}
	efft.Effect(sdin.Unmarshal[data](sdinData)).Equals("sdin.IntoForUnmarshal: sdin.TokenInto line=5 path=OutermostStruct.InnerList.1: sdin.FromSDINToken type=*sdin_test.pair: sdintest.TagWithoutSeparator token=\"c?d\"")
}
