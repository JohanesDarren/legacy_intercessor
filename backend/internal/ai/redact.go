package ai

import (
	"regexp"
	"strings"
)

// Redaction is the privacy gate required by PRD F-06 and tests T-01/T-02/T-06.
// It strips contact details and honour the display consent of each record.

var (
	// Indonesian and international phone numbers, including spaced and dashed
	// forms such as 0812-3456-7890 or +62 812 3456 7890.
	phoneRe = regexp.MustCompile(`(?i)(\+\d{1,3}[\s-]?)?(\(?0\d{2,4}\)?[\s-]?)?\d{3,4}[\s-]?\d{3,4}([\s-]?\d{2,4})?`)

	// Email addresses.
	emailRe = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)

	// Indonesian street addresses: "Jl.", "Jalan", "Gg.", "Blok", "RT/RW".
	addressRe = regexp.MustCompile(`(?i)\b(jl\.?|jalan|gg\.?|gang|blok|rt\.?\s*\d+|rw\.?\s*\d+|komplek|perumahan|apartemen|apt\.?)\b[^\n,.;]{0,60}`)

	// Social handles and URLs that could identify a person.
	urlRe = regexp.MustCompile(`(?i)\b(https?://\S+|www\.\S+|@[a-z0-9_.]{3,})`)
)

// Consent values that allow a record to appear in a summary or slide.
const (
	ConsentYes       = "yes"
	ConsentAnonymous = "anonymous"
	ConsentNo        = "no"
)

// EligibleForDisplay reports whether a record may be summarised or shown.
func EligibleForDisplay(consent string) bool {
	return consent == ConsentYes || consent == ConsentAnonymous
}

// RedactText removes contact details from free text.
func RedactText(s string) string {
	s = emailRe.ReplaceAllString(s, "[contact removed]")
	s = urlRe.ReplaceAllString(s, "[link removed]")
	s = addressRe.ReplaceAllString(s, "[address removed]")
	s = phoneRe.ReplaceAllStringFunc(s, func(match string) string {
		// Only redact strings that actually look like a phone number: at least
		// 9 digits. This avoids mangling ordinary numbers in prayer content.
		if countDigits(match) >= 9 {
			return "[phone removed]"
		}
		return match
	})
	return strings.TrimSpace(collapseSpaces(s))
}

// RedactName returns the name to send to the provider, given the consent
// value. "anonymous" always becomes a placeholder; "no" returns empty so the
// caller can drop the record entirely.
func RedactName(name string, consent string) string {
	switch consent {
	case ConsentYes:
		return strings.TrimSpace(name)
	case ConsentAnonymous:
		return "Person A"
	default:
		return ""
	}
}

func countDigits(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	return n
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
