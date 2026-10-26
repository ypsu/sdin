package sdin_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/ypsu/efftesting/efft"
	"github.com/ypsu/sdin"
)

func TestGet(t *testing.T) {
	efft.Init(t)

	n := efft.Must1(sdin.Parse(sdin.TFlatStruct(), `a:{} hello:{b:1 world:2} c:[3 4 {x+:5}]`))
	efft.Effect(n.Get("hello.world")).Equals("2")
	efft.Effect(n.Get("hello").Get("world")).Equals("2")
	efft.Effect(n.Get("hello.x")).Equals("")
	efft.Effect(n.Get("c.1")).Equals("4")
	efft.Effect(n.Get("c.2.x.0")).Equals("5")
	efft.Effect(n.Get("c.3")).Equals("")
	efft.Effect(n.Get("c.-1")).Equals("")
	efft.Effect(n.Get("")).Equals("")
	efft.Effect((*sdin.Node)(nil).Get("hello.world")).Equals("")
}

func TestReplace(t *testing.T) {
	efft.Init(t)
	n := efft.Must1(sdin.Parse(sdin.TFlatStruct(), "a:1 b:2 c:3 d:4"))
	n.Replace(func(n *sdin.Node) (*sdin.Node, error) {
		switch n.Name {
		case "b":
			return nil, nil
		case "c":
			n.Value = sdin.MakeTokenFromRepr("3r")
			return n, nil
		case "d":
			return sdin.NewNode("e", sdin.BareToken("4r")), nil
		}
		return n, nil
	})
	efft.Effect(n).Equals("a:1 c:3r e:4r")
}

func ExampleNode_Get() {
	n, err := sdin.Parse(sdin.TFlatStruct(), `
		a: {}
		b: {bb:[x y]}
		c: {}
	`)
	if err != nil {
		fmt.Printf("ParsingForGetFailed: %v", err)
		return
	}
	fmt.Println(n.Get("b.bb.1").Format())

	// Output:
	// y
}

func init() {
	os.Setenv("DATABASE", "db.example.com")
	os.Setenv("PASSWORD", "hunter2")
}

// Here's an example where we replace env() forms with values from the environment.
func ExampleNode_Replace() {
	n, err := sdin.Parse(sdin.TFlatStruct(), `
		command: 'runsql'
		arguments: {
			query:    'SELECT * FROM mytable'
			database: env(DATABASE)
			password: env(PASSWORD)
		}
	`)
	if err != nil {
		fmt.Printf("ParsingForReplaceFailed: %v", err)
		return
	}

	err = n.Replace(func(n *sdin.Node) (*sdin.Node, error) {
		if n.Kind().FormName() != "env" {
			return n, nil
		}
		if len(n.Children) != 1 || n.Children[0].Name != "" || !n.Children[0].Kind().Identifier() {
			return nil, fmt.Errorf("InvalidEnvArgs: env takes exactly one identifier arg")
		}
		value, ok := os.LookupEnv(n.Children[0].Value.Repr())
		if !ok {
			return nil, fmt.Errorf("EnvvarNotFound var=%s", n.Children[0].Value.Repr())
		}
		return sdin.NewNode("", value), nil
	})
	if err != nil {
		fmt.Printf("EnvReplaceFailed: %v", err)
		return
	}
	fmt.Print(n.Format())

	// Output:
	// command: 'runsql'
	// arguments: {
	// 	query:    'SELECT * FROM mytable'
	// 	database: 'db.example.com'
	// 	password: 'hunter2'
	// }
}
