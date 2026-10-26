// Package cmd implements the command line tool's internals.
package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ypsu/sdin"
)

// Command describes the invocation environment.
type Command struct {
	R      io.Reader
	W, Err io.Writer
	Args   []string
}

// Run executes the command line tool's logic.
func (cmd *Command) Run() error {
	var compact bool
	var filename string

	fs := flag.NewFlagSet("sdin", flag.ContinueOnError)
	fs.BoolVar(&compact, "compact", false, "Format output onto a single, compact line.")
	fs.StringVar(&filename, "filename", "", "The filename used for .sdinl detection in pipe mode.")
	fs.SetOutput(cmd.Err)
	fs.Usage = func() {
		fmt.Fprintf(cmd.Err, "Usage: sdin [flags] format <somefile >someotherfile  # pipe mode\n")
		fmt.Fprintf(cmd.Err, "Usage: sdin [flags] format file1.txt file2.txt       # rewrite mode\n")
		fmt.Fprintf(cmd.Err, "\n")
		fmt.Fprintf(cmd.Err, "Files ending in .sdinl format as lists; all others format as structs.\n")
		fmt.Fprintf(cmd.Err, "\n")
		fmt.Fprintf(cmd.Err, "Flags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(cmd.Args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("sdin.ParseFlags: %v", err)
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return nil
	}
	for _, arg := range fs.Args() {
		if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("sdin.FlagLikePositionalArg arg=%q: flags must appear before the subcommand", arg)
		}
	}
	if fs.Arg(0) != "format" {
		return fmt.Errorf("sdin.UnknownCommand cmd=%q: only format is supported", fs.Arg(0))
	}
	format := func(n *sdin.Node) string { return n.Format() }
	if compact {
		format = func(n *sdin.Node) string { return n.String() + "\n" }
	}
	if fs.NArg() == 1 {
		data, err := io.ReadAll(cmd.R)
		if err != nil {
			return fmt.Errorf("sdin.ReadInput: %v", err)
		}
		mode := sdin.TFlatStruct()
		if filepath.Ext(filename) == ".sdinl" {
			mode = sdin.TFlatList()
		}
		n, err := sdin.Parse(mode, string(data))
		if err != nil {
			return fmt.Errorf("sdin.ParseInput: %v", err)
		}
		if errs := (sdin.Sanitizer{}).Run(n); errs != nil {
			for _, err := range errs {
				fmt.Fprintf(cmd.Err, "%s\n", err)
			}
			return fmt.Errorf("sdin.UnsanitaryInput")
		}
		fmt.Fprintf(cmd.W, "%s", format(n))
		return nil
	}

	var failedFiles []string
fileloop:
	for _, filename := range fs.Args()[1:] {
		data, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(cmd.Err, "sdin.ReadFile: %v\n", err)
			failedFiles = append(failedFiles, filename)
			continue fileloop
		}
		mode := sdin.TFlatStruct()
		if filepath.Ext(filename) == ".sdinl" {
			mode = sdin.TFlatList()
		}
		n, err := sdin.Parse(mode, string(data))
		if err != nil {
			fmt.Fprintf(cmd.Err, "sdin.ParseFile file=%s: %v\n", filename, err)
			failedFiles = append(failedFiles, filename)
			continue fileloop
		}
		if errs := (sdin.Sanitizer{}).Run(n); errs != nil {
			for _, err := range errs {
				fmt.Fprintf(cmd.Err, "sdin.SanitizeFile file=%s: %s\n", filename, err)
			}
			failedFiles = append(failedFiles, filename)
			continue fileloop
		}
		if err := os.WriteFile(filename, []byte(format(n)), 0644); err != nil {
			fmt.Fprintf(cmd.Err, "sdin.WriteFile: %v\n", err)
			failedFiles = append(failedFiles, filename)
			continue fileloop
		}
	}
	if len(failedFiles) > 0 {
		return fmt.Errorf("sdin.FileFormattingFailed files=%v", failedFiles)
	}
	return nil
}
