package sdin_test

import (
	"testing"

	"github.com/ypsu/efftesting/efft"
	"github.com/ypsu/sdin"
)

func TestDirectives(t *testing.T) {
	efft.Init(t)

	n := sdin.NewNode("", sdin.TFlatList())
	n.Children = efft.Must1(sdin.Parse(sdin.TFlatStruct(), `
		#% DuplicatedDirective(1)
		#% DuplicatedDirective(2)
		Field: Value
	`)).Get("Field").Directives()
	efft.Effect(n.Format()).Equals(`
		DuplicatedDirective(1)
		DuplicatedDirective(2)
		`,
	)

	n.Children = efft.Must1(sdin.Parse(sdin.TFlatStruct(), `
		# Normal comment.
		#% somepkg.Directive1()
		#% somepkg.Directive2(with   args)
		#% somepkg.Directive3('with  single   quoted' args)
		#% somepkg.Directive4("with \" quoted" args)
		#% somepkg.Directive5("with\nnewline" 'args' arg 3 "arg \"'4'\"")
		#% somepkg.Directive6("\\")
		#% somepkg.BadDirective1(')
		#% somepkg.BadDirective2(")
		#% somepkg.EOLSpace(dir  )
		Field: Value
	`)).Get("Field").Directives()
	efft.Effect(n.Format()).Equals(`
		somepkg.Directive1()
		somepkg.Directive2(with args)
		somepkg.Directive3('with  single   quoted' args)
		somepkg.Directive4("with \" quoted" args)
		somepkg.Directive5("with\nnewline" 'args' arg 3 "arg \"'4'\"")
		somepkg.Directive6("\\")
		error('sdin.InvalidDirective line=9: sdin.TokenizeForParse: sdin.UnterminatedLiteralString line=1')
		error('sdin.InvalidDirective line=10: sdin.TokenizeForParse: sdin.UnterminatedQuotedString line=1')
		somepkg.EOLSpace(dir)
		`,
	)
}

func TestForms(t *testing.T) {
	efft.Init(t)

	var n *sdin.Node
	efft.Effect(n.Get("F").FormFields()).Equals("{}")
	efft.Effect(n.Get("F").FormValues()).Equals("[]")

	n = efft.Must1(sdin.Unmarshal[*sdin.Node](`
		F: sdin.StrJoin(
			sep:','
			'apple'
			'banana'
			'cherry'
		)
	`))
	efft.Effect(n.Get("F").FormFields()).Equals("{sep:\",\"}")
	efft.Effect(n.Get("F").FormValues()).Equals("[\"apple\" \"banana\" \"cherry\"]")
}
