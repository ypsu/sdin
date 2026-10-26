package sdin

// Interface contains the list of functions a type can implement to interface with sdin's conversion functions.
// Note that any subset of these functions can be implemented and sdin will use it if available on the type;
// they are grouped into a common interface for documentational purposes.
type Interface interface {
	FromSDINToken(Token) error
	IntoSDINToken() Token
	FromSDINNode(*Node) error
	IntoSDINNode() *Node
}

type fromNode interface {
	FromSDINNode(*Node) error
}

type intoNode interface {
	IntoSDINNode() *Node
}

type fromToken interface {
	FromSDINToken(Token) error
}

type intoToken interface {
	IntoSDINToken() Token
}
