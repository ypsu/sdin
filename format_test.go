package sdin_test

import (
	"fmt"
	"testing"

	"github.com/ypsu/efftesting/efft"
	"github.com/ypsu/sdin"
)

func TestFormat(t *testing.T) {
	efft.Init(t)

	cfg := `
		InlineObj1: {   F1:  V1 F2:V2}
		TF1:TV1

		Obj2:{F1:V1
		Field2: V3}
		Obj3: { # Opening comment.
		# Pre-closing comment.
		} # Closing comment.

		TF2: TV2 # Comment1
		Árvíztűrő: Tükörfúrógép # Comment2
		TF3: TopValue3 # Comment3


		List1: [1 2
		3 4]
		List2: [[1 2
		3 4]]
		List3: [[1 2] [3 4
		5 6]]
		List4: [1 2
		[3 4 5 6]]
		List5: [[1 2]
		[3 4 5 6]]
		List6: [[
		1 2]
		[3 4 5 6]]
		List7+: 1
		List7+: [2 3]
		List7+: {hello:world}

		TF4: map(F1 V1 Tükör # Key comment.
		Gép # Value comment.
		Another
		Value) TF5: TV6`

	n := efft.Must1(sdin.Parse(sdin.TFlatStruct(), cfg))
	efft.Effect(n.Format()).Equals(`
		
		InlineObj1: {F1:V1 F2:V2}
		TF1:        TV1

		Obj2: {
			F1:     V1
			Field2: V3
		}
		Obj3: {  # Opening comment.
			# Pre-closing comment.
		}  # Closing comment.

		TF2:       TV2           # Comment1
		Árvíztűrő: Tükörfúrógép  # Comment2
		TF3:       TopValue3     # Comment3


		List1: [1 2
			3 4]
		List2: [[1 2
			3 4]]
		List3: [[1 2] [3 4
			5 6]]
		List4: [1 2
			[3 4 5 6]]
		List5: [[1 2]
			[3 4 5 6]]
		List6: [[
				1 2]
			[3 4 5 6]]
		List7+: 1
		List7+: [2 3]
		List7+: {hello:world}

		TF4: map(
			F1 V1
			# Key comment.
			Tükör   Gép    # Value comment.
			Another Value
		)
		TF5: TV6
		`)

	efft.Effect(fmt.Sprint(n)).Equals("InlineObj1:{F1:V1 F2:V2} TF1:TV1 Obj2:{F1:V1 Field2:V3} Obj3:{} TF2:TV2 Árvíztűrő:Tükörfúrógép TF3:TopValue3 List1:[1 2 3 4] List2:[[1 2 3 4]] List3:[[1 2] [3 4 5 6]] List4:[1 2 [3 4 5 6]] List5:[[1 2] [3 4 5 6]] List6:[[1 2] [3 4 5 6]] List7+:1 List7+:[2 3] List7+:{hello:world} TF4:map(F1 V1 Tükör Gép Another Value) TF5:TV6")
}
