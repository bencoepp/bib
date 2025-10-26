package main

import (
	"bib/internal/capcheck"
	"bib/internal/capcheck/checks"
	"time"
)

func main() {
	checkers := []capcheck.Checker{
		checks.ContainerRuntimeChecker{},
		checks.KubernetesConfigChecker{},
		checks.InternetAccessChecker{
			// Use a target that works in your environment. Alternatives:
			// "https://www.google.com/generate_204" or a company endpoint.
			HTTPURL: "https://www.google.com/generate_204",
		},
		checks.ResourcesChecker{},
	}

	runner := capcheck.NewRunner(
		checkers,
		capcheck.WithGlobalTimeout(6*time.Second),
		capcheck.WithPerCheckTimeout(1*time.Second),
	)
	_ = runner
}
