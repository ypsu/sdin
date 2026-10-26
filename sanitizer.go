package sdin

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// Sanitizer checks package references and validates whether the tokens are valid.
// It also reorders sdin.KeepSorted() nodes and reformats sdin tagged strings.
type Sanitizer struct {
	// The list of allowed packages.
	// If nil, no package checks are done unless the root node preamble specifies a list with sdin.FormPackages or sdin.DirectivePackages.
	// If both this and the preamble are specified, then the allowed package list is the intersection of the two lists.
	DirectivePackages []string
	FormPackages      []string
}

func (s *Sanitizer) restrictPackages(base []string, r *Node) ([]string, error) {
	newPackages := []string{}
	for _, ch := range r.Children {
		if ch.Name != "" {
			return nil, fmt.Errorf("sdin.InvalidPackageList line=%d value=%s: *Packages() must have no names", ch.Line, ch.Value.Repr())
		}
		if ch.Value.Repr() != "." && !ch.Kind().Identifier() {
			return nil, fmt.Errorf("sdin.InvalidPackageList line=%d value=%s: value must be an identifier", ch.Line, ch.Value.Repr())
		}
		if base == nil || slices.Contains(base, ch.Value.Repr()) {
			newPackages = append(newPackages, ch.Value.Repr())
		}
	}
	return newPackages, nil
}

// Run runs the sanitizer on a given root node.
// Returns nil if there were no errors.
func (s Sanitizer) Run(n *Node) []error {
	// Extract the package allowlists from the preamble.
	directivePackages, formPackages := s.DirectivePackages, s.FormPackages
	var err error
	for _, a := range n.Directives() {
		switch a.Kind().FormName() {
		case "sdin.DirectivePackages":
			directivePackages, err = s.restrictPackages(directivePackages, a)
			if err != nil {
				return []error{err}
			}
		case "sdin.FormPackages":
			formPackages, err = s.restrictPackages(formPackages, a)
			if err != nil {
				return []error{err}
			}
		}
	}

	var dp, fp map[string]bool
	if directivePackages != nil {
		dp = make(map[string]bool, len(directivePackages))
		dp["sdin"] = true
		for _, p := range directivePackages {
			if p == "." {
				p = ""
			}
			dp[p] = true
		}
	}
	if formPackages != nil {
		fp = make(map[string]bool, len(formPackages))
		for _, p := range formPackages {
			if p == "." {
				p = ""
			}
			fp[p] = true
		}
	}
	return s.run(nil, n, ":(", dp, fp)
}

func (s *Sanitizer) run(errs []error, n *Node, parentKind Kind, directivePackages, formPackages map[string]bool) []error {
	markedInvalid := false
	switch {
	case n.Value.Repr() == "":
		errs = append(errs, fmt.Errorf("sdin.EmptyValue line=%d: empty values are not supported", n.Line))
	case n.Value.Repr() == ":(" && parentKind != ":(":
		errs = append(errs, fmt.Errorf("sdin.FlatForm line=%d: flat forms not supported", n.Line))
	case !n.Kind().Opening() && !n.Kind().String():
		for _, c := range n.Value.Repr() {
			if c == '#' || c == ' ' || c == '\t' || c == '\n' || c == ':' || c == '(' || c == '[' || c == '{' || c == ')' || c == ']' || c == '}' {
				markedInvalid = true
				errs = append(errs, fmt.Errorf("sdin.InvalidBareToken line=%d value=%q: bare tokens cannot contain colon, whitespace, or block markers", n.Line, n.Value.Repr()))
				break
			}
		}
	}
	if n.Kind().Scalar() && len(n.Children) > 0 {
		errs = append(errs, fmt.Errorf("sdin.ScalarWithChildren line=%d value=%q: scalar values cannot have children", n.Line, n.Value.Repr()))
	}
	if parentKind.Struct() {
		if n.Name == "" {
			errs = append(errs, fmt.Errorf("sdin.InvalidFieldName line=%d: missing struct field name", n.Line))
		}
		if !markedInvalid {
			for i, c := range n.Name {
				if i == 0 && '0' <= c && c <= '9' {
					errs = append(errs, fmt.Errorf("sdin.InvalidFieldName line=%d value=%q: struct field names cannot start with digits", n.Line, n.Name))
					break
				}
				if c != '_' && !unicode.IsDigit(c) && !unicode.IsLetter(c) {
					errs = append(errs, fmt.Errorf("sdin.InvalidFieldName line=%d value=%q: struct field names must contain only underscores, digits, and letters", n.Line, n.Name))
					break
				}
			}
		}
	} else if parentKind.List() && n.Name != "" {
		errs = append(errs, fmt.Errorf("sdin.UnexpectedName line=%d value=%q: list values cannot have names", n.Line, n.Value.Repr()))
	} else if n.Name != "" {
		for _, c := range n.Name {
			if c == '#' || c == ' ' || c == '\t' || c == '\n' || c == ':' || c == '(' || c == '[' || c == '{' || c == ')' || c == ']' || c == '}' {
				errs = append(errs, fmt.Errorf("sdin.InvalidName line=%d value=%q: names cannot contain colon, whitespace, or block markers", n.Line, n.Name))
				break
			}
		}
	}

	if n.Value.Repr() == "map(" {
		keys := map[string]int{}
		for i := 0; i < len(n.Children); i += 2 {
			k := n.Children[i]
			if line, found := keys[k.Value.Repr()]; found {
				errs = append(errs, fmt.Errorf("sdin.DuplicateMapKey line1=%d line2=%d key=%s", line, k.Line, k.Value.Repr()))
			}
			keys[k.Value.Repr()] = k.Line
		}
	}
	if n.Value.Repr() != "map(" && formPackages != nil && n.Kind().Form() && !formPackages[n.Kind().PackageName()] {
		errs = append(errs, fmt.Errorf("sdin.UndefinedFormPackage line=%d package=%s symbol=%s", n.Line, n.Kind().PackageName(), n.Value.Repr()))
	}
	directives := n.Directives()
	for _, a := range directives {
		if !a.Kind().Form() {
			errs = append(errs, fmt.Errorf("sdin.NonFormDirective line=%d value=%s: directives must be forms", a.Line, a.Value.Repr()))
		} else if a.Kind().FormName() == "error" && len(a.Children) == 1 && a.Children[0].Kind().String() {
			if msg, err := a.Children[0].As[string](); err == nil { // on success
				errs = append(errs, fmt.Errorf("%s", msg))
			} else {
				errs = append(errs, fmt.Errorf("%s", a.Children[0].Value.s))
			}
		}
	}
	for _, a := range directives {
		if directivePackages != nil {
			if a.Kind().FormName() == "error" && len(a.Children) == 1 && a.Children[0].Kind().String() {
				continue
			}
			if a.Kind().Form() && !directivePackages[a.Kind().PackageName()] {
				errs = append(errs, fmt.Errorf("sdin.UndefinedDirectivePackage line=%d package=%s name=%s", a.Line, a.Kind().PackageName(), a.Kind().FormName()))
			}
		}
		if a.Kind().Form() && a.Kind().PackageName() == "sdin" {
			switch a.Kind().FormName() {
			case "sdin.DirectivePackages", "sdin.FormPackages":
				if parentKind != ":(" {
					errs = append(errs, fmt.Errorf("sdin.InvalidSdinDirective line=%d: DirectivePackages() and FormPackages() can appear only in the preamble", a.Line))
				}
			case "sdin.KeepSorted":
				if !n.Kind().Opening() {
					errs = append(errs, fmt.Errorf("sdin.InvalidKeepSortedNode line=%d: KeepSorted() must be in front of a block, not scalar", a.Line))
					break
				}
				if n.Kind().Form() {
					errs = append(errs, fmt.Errorf("sdin.UnsupportedFormSorting line=%d: KeepSorted() does not support sorting form values", a.Line))
					break
				}

				tosort := n.Children
				for _, arg := range a.Children {
					switch arg.Name {
					case "path":
						tn := n.Get(arg.Value.Repr())
						if tn == nil {
							errs = append(errs, fmt.Errorf("sdin.InvalidKeepSortedPath line=%d path=%s", a.Line, arg.Value.Repr()))
							tosort = nil
						} else {
							if tn.Kind().Form() {
								errs = append(errs, fmt.Errorf("sdin.UnsupportedFormSorting line=%d: KeepSorted() does not support sorting form values", a.Line))
								tosort = nil
								break
							}
							tosort = tn.Children
						}
					default:
						errs = append(errs, fmt.Errorf("sdin.InvalidKeepSortedParameter line=%d param=%s", a.Line, arg.Name))
					}
				}
				slices.SortStableFunc(tosort, cmpNodes)
			default:
				errs = append(errs, fmt.Errorf("sdin.UnknownSdinDirective line=%d form=%s", a.Line, a.Kind().FormName()))
			}
		}
	}

	// Reformat sdin tagged strings.
	if st, err := n.Value.As[StringToken](); err == nil && st.Tag() == "sdin" {
		func() {
			nn, err := Parse(TFlatForm(), st.Data)
			if err != nil {
				errs = append(errs, fmt.Errorf("sdin.ReformatInnerSdin line=%d: %v", n.Line, err))
				return
			}
			if err := (Sanitizer{}).Run(nn); err != nil {
				errs = append(errs, fmt.Errorf("sdin.SanitizeInnerSdin line=%d: %v", n.Line, err[0]))
				return
			}
			newdata := nn.Format()
			if newdata == st.Data && !strings.HasSuffix(st.Quote, "|") {
				return
			}
			st.Quote = strings.TrimSuffix(st.Quote, "|")
			st.Data = newdata
			n.Value = MakeToken(st)
		}()
	}

	for _, ch := range n.Children {
		errs = s.run(errs, ch, n.Kind(), directivePackages, formPackages)
	}
	return errs
}

func cmpNodes(a, b *Node) int {
	if d := cmp.Compare(a.Name, b.Name); d != 0 {
		return d
	}
	va, vb := a.Value.s, b.Value.s
	if s, err := a.Value.As[string](); err == nil {
		va = s
	}
	if s, err := b.Value.As[string](); err == nil {
		vb = s
	}
	if d := cmp.Compare(va, vb); d != 0 {
		return d
	}
	for i := range min(len(a.Children), len(b.Children)) {
		if d := cmpNodes(a.Children[i], b.Children[i]); d != 0 {
			return d
		}
	}
	return len(a.Children) - len(b.Children)
}
