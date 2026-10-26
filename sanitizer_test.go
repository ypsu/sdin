package sdin_test

import (
	"strings"
	"testing"

	"github.com/ypsu/efftesting/efft"
	"github.com/ypsu/sdin"
)

func TestSanitizer(t *testing.T) {
	efft.Init(t)
	root := sdin.NewNode("", sdin.TFlatStruct())
	n := sdin.NewNode("name", "value")
	root.Children = append(root.Children, n, sdin.NewNode("map", sdin.TForm("map")), sdin.NewNode("list", sdin.TList()))
	mn := sdin.NewNode("name", "value")
	root.Children[1].Children = append(root.Children[1].Children, mn)
	run := func(s sdin.Sanitizer) string {
		errs := s.Run(root)
		if len(errs) == 0 {
			return ""
		}
		if len(errs) == 1 {
			return errs[0].Error()
		}
		w := &strings.Builder{}
		for _, e := range errs {
			w.WriteString(e.Error())
			w.WriteByte('\n')
		}
		return w.String()
	}

	defaultSanitizer := sdin.Sanitizer{}
	efft.Effect(run(defaultSanitizer)).Equals("")

	efft.Note = "InvalidValueTokens"
	n.Value = sdin.MakeTokenFromRepr("value with space")
	efft.Effect(run(defaultSanitizer)).Equals("sdin.InvalidBareToken line=0 value=\"value with space\": bare tokens cannot contain colon, whitespace, or block markers")
	n.Value = sdin.MakeTokenFromRepr("val:ue")
	efft.Effect(run(defaultSanitizer)).Equals("sdin.InvalidBareToken line=0 value=\"val:ue\": bare tokens cannot contain colon, whitespace, or block markers")
	n.Value = sdin.MakeTokenFromRepr("val(ue")
	efft.Effect(run(defaultSanitizer)).Equals("sdin.InvalidBareToken line=0 value=\"val(ue\": bare tokens cannot contain colon, whitespace, or block markers")
	n.Value = sdin.MakeTokenFromRepr("value")

	efft.Note = "UndetectedInvalidTokens" // missing implementation, maybe add support later
	n.Value = sdin.MakeTokenFromRepr("val((")
	efft.Effect(run(defaultSanitizer)).Equals("")
	n.Value = sdin.MakeTokenFromRepr("val{{")
	efft.Effect(run(defaultSanitizer)).Equals("")
	n.Value = sdin.MakeTokenFromRepr("val[[")
	efft.Effect(run(defaultSanitizer)).Equals("")
	n.Value = sdin.MakeTokenFromRepr("value")

	efft.Note = "InvalidStructFieldNames"
	n.Name = ""
	efft.Effect(run(defaultSanitizer)).Equals("sdin.InvalidFieldName line=0: missing struct field name")
	n.Name = "name:"
	efft.Effect(run(defaultSanitizer)).Equals("sdin.InvalidFieldName line=0 value=\"name:\": struct field names must contain only underscores, digits, and letters")
	n.Name = "name("
	efft.Effect(run(defaultSanitizer)).Equals("sdin.InvalidFieldName line=0 value=\"name(\": struct field names must contain only underscores, digits, and letters")
	n.Name = "name.Name"
	efft.Effect(run(defaultSanitizer)).Equals("sdin.InvalidFieldName line=0 value=\"name.Name\": struct field names must contain only underscores, digits, and letters")
	n.Name = "name"

	efft.Note = "InvalidFormFieldNames"
	mn.Name = ""
	efft.Effect(run(defaultSanitizer)).Equals("")
	mn.Name = "name:"
	efft.Effect(run(defaultSanitizer)).Equals("sdin.InvalidName line=0 value=\"name:\": names cannot contain colon, whitespace, or block markers")
	mn.Name = "name("
	efft.Effect(run(defaultSanitizer)).Equals("sdin.InvalidName line=0 value=\"name(\": names cannot contain colon, whitespace, or block markers")
	mn.Name = "name.Name"
	efft.Effect(run(defaultSanitizer)).Equals("")
	mn.Name = "name"

	efft.Note = "InvalidForms"
	n.Value = sdin.MakeTokenFromRepr("unknownBuiltin(")
	efft.Effect(run(defaultSanitizer)).Equals("")
	efft.Effect(run(sdin.Sanitizer{FormPackages: []string{"somepkg"}})).Equals("sdin.UndefinedFormPackage line=0 package= symbol=unknownBuiltin(")
	efft.Effect(run(sdin.Sanitizer{FormPackages: []string{".", "somepkg"}})).Equals("")
	n.Value = sdin.MakeTokenFromRepr("sdin.SomeForm(")
	efft.Effect(run(defaultSanitizer)).Equals("")
	n.Value = sdin.MakeTokenFromRepr("somepkg.SomeForm(")
	efft.Effect(run(defaultSanitizer)).Equals("")
	efft.Effect(run(sdin.Sanitizer{FormPackages: []string{"somepkg"}})).Equals("")
	efft.Effect(run(sdin.Sanitizer{FormPackages: []string{"otherpkg"}})).Equals("sdin.UndefinedFormPackage line=0 package=somepkg symbol=somepkg.SomeForm(")
	root.Pre = []string{"#% sdin.FormPackages(somepkg)"}
	efft.Effect(run(defaultSanitizer)).Equals("")
	efft.Effect(run(sdin.Sanitizer{FormPackages: []string{"somepkg"}})).Equals("")
	efft.Effect(run(sdin.Sanitizer{FormPackages: []string{"otherpkg"}})).Equals("sdin.UndefinedFormPackage line=0 package=somepkg symbol=somepkg.SomeForm(")
	n.Value = sdin.MakeTokenFromRepr("otherpkg.SomeForm(")
	efft.Effect(run(defaultSanitizer)).Equals("sdin.UndefinedFormPackage line=0 package=otherpkg symbol=otherpkg.SomeForm(")
	n.Value = sdin.MakeTokenFromRepr("unknownBuiltin(")
	efft.Effect(run(defaultSanitizer)).Equals("sdin.UndefinedFormPackage line=0 package= symbol=unknownBuiltin(")
	root.Pre = []string{"#% sdin.FormPackages(. somepkg)"}
	efft.Effect(run(defaultSanitizer)).Equals("")
	n.Value = sdin.MakeTokenFromRepr("value")
	root.Pre = nil

	efft.Note = "InvalidDirectives"
	n.Post = "#% somepkg.NotForm"
	efft.Effect(run(defaultSanitizer)).Equals("sdin.NonFormDirective line=1 value=somepkg.NotForm: directives must be forms")
	n.Post = ""
	root.Children[2].Post = "#% sdin.KeepSorted()"
	efft.Effect(run(defaultSanitizer)).Equals("")
	root.Children[2].Post = ""
	n.Post = "#% sdin.SomeForm()"
	efft.Effect(run(defaultSanitizer)).Equals("sdin.UnknownSdinDirective line=1 form=sdin.SomeForm")
	n.Post = "#% somepkg.SomeForm()"
	efft.Effect(run(defaultSanitizer)).Equals("")
	efft.Effect(run(sdin.Sanitizer{DirectivePackages: []string{"somepkg"}})).Equals("")
	efft.Effect(run(sdin.Sanitizer{DirectivePackages: []string{"otherpkg"}})).Equals("sdin.UndefinedDirectivePackage line=1 package=somepkg name=somepkg.SomeForm")
	root.Pre = []string{"#% sdin.DirectivePackages(sdin somepkg)"}
	root.Children[2].Post = "#% sdin.KeepSorted()"
	efft.Effect(run(defaultSanitizer)).Equals("")
	root.Children[2].Post = ""
	n.Post = "#% somepkg.SomeForm()"
	efft.Effect(run(defaultSanitizer)).Equals("")
	efft.Effect(run(sdin.Sanitizer{DirectivePackages: []string{"sdin", "somepkg"}})).Equals("")
	efft.Effect(run(sdin.Sanitizer{DirectivePackages: []string{"sdin", "otherpkg"}})).Equals("sdin.UndefinedDirectivePackage line=1 package=somepkg name=somepkg.SomeForm")
	n.Post = "#% error('example error message')"
	efft.Effect(run(defaultSanitizer)).Equals("example error message")
	root.Pre = nil
	n.Post = ""

	efft.Note = "PreambleDirectiveErrors"
	root = efft.Must1(sdin.Unmarshal[*sdin.Node](`# Header
		#% sdin.KeepSorted(
		#% sdin.BadForm(

		x:1
	`))
	efft.Effect(run(defaultSanitizer)).Equals(`
		sdin.InvalidDirective line=2: sdin.UnclosedNode startline=1
		sdin.InvalidDirective line=3: sdin.UnclosedNode startline=1
		`)
}
