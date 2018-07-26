//
// This file contains a helper method to allow parsing #include
// lines from static-resources included in our binary.
//

package main

import (
	"fmt"
	"strings"
)

//
// CACHE is a cache for our expanded templates.
//
var CACHE = make(map[string]string)

// ExpandResource reads a file from our static-resources, and
// processes any lines that contain "includes".
//
// This is done to ensure that the HTML that we serve to clients
// doesn't require any CSS or JS requests
//
func ExpandResource(file string) (string, error) {

	//
	// Return from the cache, if we can.
	//
	tmp := CACHE[file]
	if tmp != "" {
		return tmp, nil
	}

	//
	// Get the master template
	//
	data, err := getResource(file)
	if err != nil {
		return "", err
	}

	//
	// Build up the output here, by expanding `#include` entries.
	//
	output := ""

	// look for the "#include <xx>" lines
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {

		// If we found such a line
		if strings.HasPrefix(line, "#include ") {

			// Get the file which should be handled
			inc := line
			inc = strings.TrimPrefix(inc, "#include ")

			// Look that up.
			txt, err := getResource("data/" + inc)
			if err != nil {
				fmt.Printf("Failed to read 'data/" + inc + "'")
				return "", nil
			}

			// If it succeeded then we'll add the contents
			// to the output.
			//
			// NOTE: We don't allow recursion, an #include'd
			// file cannot #include another.
			//
			output += string(txt)
		} else {
			//
			// Otherwise add the literal contents
			// and the newline we missed.
			output += line
			output += "\n"
		}
	}

	//
	// Save the expanded text in our cache.
	//
	CACHE[file] = output

	//
	// Before returning it.
	//
	return output, nil
}
