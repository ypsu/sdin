// Command sdin is the command-line interface to sdin related utilities.
package main

import (
	"fmt"
	"os"

	"github.com/ypsu/sdin/cmd"
)

func main() {
	cmd := cmd.Command{R: os.Stdin, W: os.Stdout, Err: os.Stderr, Args: os.Args[1:]}
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
