package sdin_test

import (
	"testing"

	"github.com/ypsu/efftesting/efft"
	"github.com/ypsu/sdin"
)

func TestParse(t *testing.T) {
	efft.Init(t)

	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x:[1 2] y:{a:b}`)).Equals("x:[1 2] y:{a:b}")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x:[1 2}`)).Equals("sdin.InvalidBlockClose line=1 startline=1 gotchar=} wantchar=]")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x:[1 2`)).Equals("sdin.UnclosedNode startline=1")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x:[1 2]]`)).Equals("sdin.InvalidTopBlockClose line=1")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x:`)).Equals("sdin.EOFAfterName line=1")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x:#hello`)).Equals("sdin.NewlineAfterName line=1: key names and values must be on the same line")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x:{:}`)).Equals("sdin.EmptyName line=1: name must be non-empty")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x:{y:}`)).Equals("sdin.MissingValue line=1: found block closer instead of value")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x:{y}`)).Equals("sdin.UnnamedStructMember line=1 value=y: all struct members must be name-value pairs (or are you missing a colon?)")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x`)).Equals("sdin.UnnamedStructMember line=1 value=x: all struct members must be name-value pairs (or are you missing a colon?)")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x:1 x:2`)).Equals("sdin.RedefinedField line=1 name=\"x\" prevline=1")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x..x:1`)).Equals("sdin.InvalidStructFieldName line=1 name=\"x..x\": struct field name must be a dot separated identifier path")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x..:1`)).Equals("sdin.InvalidStructFieldName line=1 name=\"x..\": struct field name must be a dot separated identifier path")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x.x.x:1 x.x:2`)).Equals("sdin.RedefinedPrevField line=1 name=x.x prevname=x.x.x")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x+:1 y:2 x:3`)).Equals("sdin.RedefinedField line=1 name=\"x\" prevline=1")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x+:1 y:2 x+:3`)).Equals("sdin.RedefinedFlatField line=1 name=\"x\" prevline=1")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x.x:1 x.x.x:2`)).Equals("sdin.RedefinedFlatField line=1 name=\"x.x\" prevline=1")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x-y:1`)).Equals("sdin.InvalidStructFieldName line=1 name=\"x-y\": struct field name must be a dot separated identifier path")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), `x.y-z:1`)).Equals("sdin.InvalidStructFieldName line=1 name=\"x.y-z\": struct field name must be a dot separated identifier path")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), "A:1\r\n")).Equals("A:1")
	efft.Effect(sdin.Parse(sdin.TFlatStruct(), "A:1\r\nB:'x'\r\n")).Equals("A:1 B:\"x\"")
}
