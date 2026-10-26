package sdin_test

import (
	"fmt"

	"github.com/ypsu/sdin"
)

const rawListSDIN = `# This is an sdin file that contains only values.
"alice"
"bob"
"charlie"
"dave"
`

const rawStructSDIN = `# This sdin file represents a collection of items.
Items+: {
	Name: 'Independence Day'
	Kind: DVD
	Location: 'living room bookshelf'
}
Items+: {
	Name: 'Teddy Bear'
	Kind: Toy
	Location: 'my bed'
	Comment: '''
		This is my best friend.
		I cannot sleep without it.
		'''
}
`

type item struct {
	Name     string
	Kind     sdin.Identifier
	Location string
	Comment  string
}

func Example() {
	var names []string
	var err error

	// Unmarshaling a list.
	names, err = sdin.Unmarshal[[]string](rawListSDIN)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Input list has %d names, starting with %q, ending with %q.\n", len(names), names[0], names[len(names)-1])

	// Marshaling a list.
	fmt.Printf("sdin formatted version of the list:\n%s\n", sdin.Marshal(names))

	// Simple AST manipulation: add a new item to the collection.
	n, err := sdin.Unmarshal[*sdin.Node](rawStructSDIN)
	if err != nil {
		panic(err)
	}
	items := n.Get("Items")
	items.Children = append(items.Children, sdin.NewNode("", item{
		Name:     "Toothbrush",
		Kind:     "Consumable",
		Location: "bathroom",
		Comment:  "Mine is the blue one.",
	}))
	fmt.Printf("New, edited sdin:\n%s\n", n.Format())

	// Fetching a specific field from an item.
	var teddyComment string
	teddyComment, err = items.Children[1].Get("Comment").As[string]()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Teddy comment:\n%s", teddyComment)

	// Output:
	// Input list has 4 names, starting with "alice", ending with "dave".
	// sdin formatted version of the list:
	// 'alice'
	// 'bob'
	// 'charlie'
	// 'dave'
	//
	// New, edited sdin:
	// # This sdin file represents a collection of items.
	// Items+: {
	// 	Name:     'Independence Day'
	// 	Kind:     DVD
	// 	Location: 'living room bookshelf'
	// }
	// Items+: {
	// 	Name:     'Teddy Bear'
	// 	Kind:     Toy
	// 	Location: 'my bed'
	// 	Comment: '''
	// 		This is my best friend.
	// 		I cannot sleep without it.
	// 		'''
	// }
	// Items+: {
	// 	Name:     'Toothbrush'
	// 	Kind:     Consumable
	// 	Location: 'bathroom'
	// 	Comment:  'Mine is the blue one.'
	// }
	//
	// Teddy comment:
	// This is my best friend.
	// I cannot sleep without it.
}
