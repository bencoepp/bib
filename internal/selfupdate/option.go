package selfupdate

import "time"

type Option struct {
	Owner           string
	Repo            string
	BinaryName      string
	Version         string
	AllowPrerelease bool
	HTTPTimeout     time.Duration
}
