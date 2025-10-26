package main

import "bib/cmd/bib/cmd"

func main() {
	// Pass the version from package main into the Cobra package.
	cmd.AppVersion = Version
	cmd.Execute()
}
