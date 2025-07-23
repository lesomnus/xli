package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lesomnus/xli/internal/examples/completion/cmd"
)

func main() {
	if err := cmd.NewExampleCompletionCmd().Run(context.TODO(), os.Args[1:]); err != nil {
		fmt.Printf("err: %v\n", err)
	}
}
