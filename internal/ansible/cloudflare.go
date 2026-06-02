package ansible

import (
	"io"
)

// EnsureWildcardCertificate triggers a playbook to ensure a wildcard certificate exists for the given domain.
func EnsureWildcardCertificate(dryRun bool, domain string, output io.Writer) (*Result, error) {
	extraVars := map[string]string{
		"target_domain": domain,
	}

	// We use the same runner logic as other ops
	return runPlaybook("ensure-wildcard.yml", "", dryRun, nil, extraVars, output, false)
}
