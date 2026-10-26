package sdin_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/ypsu/efftesting/efft"
	"github.com/ypsu/sdin"
)

func stringifyIdentifiers(n *sdin.Node) {
	var id sdin.Identifier
	id, err := n.Value.As[sdin.Identifier]()
	if err == nil && id != "false" && id != "true" { // on success
		n.Value = sdin.MakeToken(string(id))
	}
	for _, ch := range n.Children {
		stringifyIdentifiers(ch)
	}
}

func TestJSON(t *testing.T) {
	efft.Init(t)

	type commit struct {
		Author      sdin.Identifier
		Time        time.Time
		Description string
		Files       []string
		Issues      []int64
		SizeKB      float64
		Tested      bool
		Tags        map[sdin.Identifier]string
		CustomTags  map[sdin.BareToken]string
	}

	n := efft.Must1(sdin.Parse(sdin.TFlatStruct(), `
		Author:        alice
		Time:          2016-01-02.121314
		Files+:       'file1.txt'
		Files+:       "file2.txt"
		Issues:        [123 234]
		SizeKB:        34.500
		Tested:        true
		Tags.safe:     'true'
		Tags.testlink: "test/123456"

		CustomTags: map(
			hello-world 'hi'
		)

		Description: '''
			Hello world!
			This is a commit!
			'''
	`))

	efft.Note = "parse in Go struct"
	acommit := efft.Must1(n.As[commit]())
	efft.Effect(acommit).Equals(`
		{
		  "Author": "alice",
		  "Time": "2016-01-02T12:13:14Z",
		  "Description": "Hello world!\nThis is a commit!\n",
		  "Files": [
		    "file1.txt",
		    "file2.txt"
		  ],
		  "Issues": [
		    123,
		    234
		  ],
		  "SizeKB": 34.5,
		  "Tested": true,
		  "Tags": {
		    "safe": "true",
		    "testlink": "test/123456"
		  },
		  "CustomTags": {
		    "hello-world": "hi"
		  }
		}`)

	efft.Note = "marshal raw sdin Node into json but identifiers are not supported"
	efft.Effect(n.MarshalJSON()).Equals("sdin.ParseValue line=2: sdin.ParseJSONFloat: sdin.NotANumberToken value=\"alice\"")

	efft.Note = "marshal raw sdin Node into json but with stringified identifiers"
	stringifyIdentifiers(n)
	efft.Effect(n.MarshalJSON()).Equals(`
		{
			"Author": "alice",
			"Time": 1451736794000,
			"Files": [
				"file1.txt",
				"file2.txt"
			],
			"Issues": [
				123,
				234
			],
			"SizeKB": 34.500,
			"Tested": true,
			"Tags": {
				"safe": "true",
				"testlink": "test/123456"
			},
			"CustomTags": {
				"hello-world": "hi"
			},
			"Description": "Hello world!\nThis is a commit!\n"
		}`)

	efft.Note = "json.Marshal()"
	p := efft.Must1(json.Marshal(n))
	efft.Effect(p).Equals("{\"Author\":\"alice\",\"Time\":1451736794000,\"Files\":[\"file1.txt\",\"file2.txt\"],\"Issues\":[123,234],\"SizeKB\":34.500,\"Tested\":true,\"Tags\":{\"safe\":\"true\",\"testlink\":\"test/123456\"},\"CustomTags\":{\"hello-world\":\"hi\"},\"Description\":\"Hello world!\\nThis is a commit!\\n\"}")
	q := &bytes.Buffer{}
	efft.Must(json.Indent(q, p, "", "\t"))
	efft.Effect(q).Equals(`
		{
			"Author": "alice",
			"Time": 1451736794000,
			"Files": [
				"file1.txt",
				"file2.txt"
			],
			"Issues": [
				123,
				234
			],
			"SizeKB": 34.500,
			"Tested": true,
			"Tags": {
				"safe": "true",
				"testlink": "test/123456"
			},
			"CustomTags": {
				"hello-world": "hi"
			},
			"Description": "Hello world!\nThis is a commit!\n"
		}`)

	efft.Note = "marshal to Go struct sdin and then into json"
	nn := sdin.NewNode("", acommit)
	stringifyIdentifiers(nn)
	js := efft.Must1(nn.MarshalJSON())
	efft.Effect(js).Equals(`
		{
			"Author": "alice",
			"Time": 1451736794000,
			"Description": "Hello world!\nThis is a commit!\n",
			"Files": [
				"file1.txt",
				"file2.txt"
			],
			"Issues": [
				123,
				234
			],
			"SizeKB": 34.5,
			"Tested": true,
			"Tags": {
				"safe": "true",
				"testlink": "test/123456"
			},
			"CustomTags": {
				"hello-world": "hi"
			}
		}`)

	efft.Note = "unmarshal invalid json"
	var n1 sdin.Node
	efft.Effect(n1.UnmarshalJSON([]byte(`
		{
			"Description": "Hello world!\nThis is a commit!\n",
			"Author": alice
		}
		`))).Equals("sdin.DecodeJSON line=4: sdin.UnmarshalJSONObject key=\"Author\": sdin.GetNextJSONToken: invalid character 'a' looking for beginning of value")
	efft.Effect(json.Unmarshal([]byte(`
		{
			"Description": "Hello world!\nThis is a commit!\n",
			"Author": alice
		}
		`), &n1)).Equals("invalid character 'a' looking for beginning of value")
	efft.Effect(n1.UnmarshalJSON([]byte(`
		{
			"Description": "Hello world!\nThis is a commit!\n",
			"Author": "alice"
		]
		`))).Equals("sdin.DecodeJSON line=5: sdin.ReadJSONObjectCloseToken: invalid character ']' after object key:value pair")
	efft.Effect(n1.UnmarshalJSON([]byte(`
		{
			"Author": "bob"
		},
		{
			"Author": "charlie"
		}
		`))).Equals("sdin.PartialJSONDecode: input had more than one value")

	efft.Note = "unmarshal valid json"
	efft.Must(json.Unmarshal(js, &n1))
	for i, t := range n1.Get("CustomTags").Children { // fix the type of the map keys manually
		if i%2 == 0 {
			t.Value = sdin.MakeToken(sdin.BareToken(efft.Must1(t.As[string]())))
		}
	}
	efft.Effect(n1.Format()).Equals(`
		{
			Author:      "alice"
			Time:        1451736794000
			Description: "Hello world!\nThis is a commit!\n"
			Files: [
				"file1.txt"
				"file2.txt"
			]
			Issues: [
				123
				234
			]
			SizeKB: 34.5
			Tested: true
			Tags: {
				safe:     "true"
				testlink: "test/123456"
			}
			CustomTags: map(
				hello-world "hi"
			)
		}
		`)
	// Readd type info.
	n1.Get("Author").Value = sdin.MakeToken(sdin.Identifier("alice"))
	unixms := efft.Must1(n1.Get("Time").Value.As[int64]())
	n1.Get("Time").Value = sdin.MakeToken(time.UnixMilli(unixms).UTC())
	efft.Effect(efft.Diff(efft.Stringify(acommit), efft.Stringify(n1.As[commit]()))).Equals("")

	efft.Note = "unmarshal non-identifier keyed map"
	var n2 sdin.Node
	efft.Must(n2.UnmarshalJSON([]byte(`
		{
			"Author": "alice",
			"Tags": {
				"some-tag": true
			}
		}`)))
	efft.Effect(n2.Format()).Equals(`
		{
			Author: "alice"
			Tags: map(
				"some-tag" true
			)
		}
		`)

	efft.Note = "json unmarshal map parsing"
	n = &sdin.Node{}
	efft.Must(json.Unmarshal([]byte(`{"not-identifier": {"a":1, "b":[1,2,3]}}`), n))
	efft.Effect(n).Equals("map(\"not-identifier\" {a:1 b:[1 2 3]})")

	efft.Note = "json roundtrip"
	js = []byte(`{
		"List":   [1, 2, "string"],
		"Struct": {"Key": "Value"},
		"": "",
		"Bool":   true,
		"NilObj": null
	}`)
	n = &sdin.Node{}
	efft.Must(n.UnmarshalJSON(js))
	efft.Effect(n).Equals("{List:[1 2 \"string\"] Struct:{Key:\"Value\"} __MissingNameError__:\"\" Bool:true NilObj:nil}")
	efft.Effect(n.MarshalJSON()).Equals(`
		{
			"List": [
				1,
				2,
				"string"
			],
			"Struct": {
				"Key": "Value"
			},
			"": "",
			"Bool": true,
			"NilObj": null
		}`)
}
