// Command admin manages admin users (create-user, reset-password). Owner: worker M1.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "admin CLI not implemented yet (see docs/PRD.md S5)")
	os.Exit(1)
}
