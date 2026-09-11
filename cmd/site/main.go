// Command site builds perch's documentation site out of docs/. It needs macOS
// and swiftc: a page's menu preview is the Swift perch emits, compiled and run.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/orochi235/perch/internal/site"
)

func main() {
	root := flag.String("C", ".", "repo directory to build from")
	out := flag.String("o", "_site", "directory to build into")
	serve := flag.String("serve", "", "address to serve the built site on, e.g. [::]:8080")
	flag.Parse()

	if err := site.Build(*root, *out); err != nil {
		fmt.Fprintln(os.Stderr, "site:", err)
		os.Exit(1)
	}
	fmt.Println("built", *out)

	if *serve == "" {
		return
	}
	fmt.Printf("serving %s on http://%s\n", *out, *serve)
	// Both loopbacks answer only when the address is unset or [::]; 127.0.0.1
	// and localhost each pick one family and refuse the other.
	if err := http.ListenAndServe(*serve, http.FileServer(http.Dir(*out))); err != nil {
		fmt.Fprintln(os.Stderr, "site:", err)
		os.Exit(1)
	}
}
