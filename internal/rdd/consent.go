package rdd

import "fmt"

// ConsentResult is the contract-based consent envelope for `review start`
// (spec review-consent). There is no stdin/TUI prompt: callers pass
// --consent explicitly, and relay mode returns this envelope so the caller
// can re-invoke with granted or declined.
type ConsentResult struct {
	Headline string `json:"headline"`
	Reason   string `json:"reason"`
	Evidence string `json:"evidence"`
	Granted  bool   `json:"granted"`
}

// ConsentFlag is the parsed value of the --consent flag on `review start`.
type ConsentFlag string

const (
	// ConsentRelay outputs a ConsentResult envelope without freezing the
	// candidate; the caller must re-invoke with granted or declined.
	ConsentRelay ConsentFlag = "relay"
	// ConsentGranted freezes the candidate, records consent evidence, and
	// transitions to reviewing.
	ConsentGranted ConsentFlag = "granted"
	// ConsentDeclined must not freeze the candidate or transition state.
	ConsentDeclined ConsentFlag = "declined"
)

// ParseConsentFlag parses the --consent flag value. It fails closed: any
// value other than relay, granted, or declined (including empty string)
// returns an error rather than defaulting to a permissive value (spec
// review-consent: "Flag required").
func ParseConsentFlag(s string) (ConsentFlag, error) {
	switch ConsentFlag(s) {
	case ConsentRelay:
		return ConsentRelay, nil
	case ConsentGranted:
		return ConsentGranted, nil
	case ConsentDeclined:
		return ConsentDeclined, nil
	default:
		return "", fmt.Errorf("rdd: invalid --consent value %q, want relay|granted|declined", s)
	}
}
