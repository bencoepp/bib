package checks

import (
	"bib/internal/capcheck"
	"context"
	"crypto/x509"
	"os"
	"runtime"
)

type TLSPKIChecker struct{}

func (t TLSPKIChecker) ID() capcheck.CapabilityID { return "tls_pki" }
func (t TLSPKIChecker) Description() string {
	return "Reports basic system trust store properties."
}

func (t TLSPKIChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      t.ID(),
		Name:    "TLS / PKI",
		Details: map[string]any{},
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		res.Error = "failed to load system cert pool: " + err.Error()
		return res
	}
	res.Details["system_roots"] = len(pool.Subjects())
	res.Details["goos"] = runtime.GOOS
	if path := os.Getenv("SSL_CERT_FILE"); path != "" {
		res.Details["ssl_cert_file"] = path
	}
	res.Supported = true
	return res
}
