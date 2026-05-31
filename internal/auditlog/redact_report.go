// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package auditlog

import (
	"regexp"
	"strings"
)

// Report-time (share) redaction. This is a SECOND, stronger pass applied only
// when a report is exported with --share. The write-time DefaultRedact strips
// credentials so the local log is safe at rest; RedactForShare additionally
// removes deployment-identifying data — $HOME paths, usernames, AWS ARNs, and
// 12-digit account ids — so a report is safe to paste into a chat or issue.
//
// It errs toward over-redaction (same philosophy as DefaultRedact): masking a
// benign 12-digit number is harmless; leaking an account id is not.

const shareMask = "***"

var (
	// A whole AWS ARN, masked to "arn:***" — the body carries account id,
	// region, and resource path, none of which are safe to share.
	arnRE = regexp.MustCompile(`arn:[a-zA-Z0-9-]+:[^\s"';]*`)
	// A bare 12-digit AWS account id. \b on both sides so an 11- or 13-digit
	// run isn't partially masked.
	accountIDRE = regexp.MustCompile(`\b\d{12}\b`)
	// A /Users/<name> or /home/<name> segment — the <name> is a username. The
	// $HOME prefix is collapsed to ~ first (see anonymizeText); this catches
	// OTHER users' homes and the case where HOME is unset.
	userSegmentRE = regexp.MustCompile(`(/Users|/home)/[^/\s"';]+`)
)

// anonymizeText collapses the caller's $HOME to "~" and replaces any remaining
// /Users/<name> or /home/<name> username segment with /Users/<user>. It works
// on whole paths (CWD) and on paths embedded in free text (Command, Reason).
func anonymizeText(s, home string) string {
	if home != "" {
		s = strings.ReplaceAll(s, home, "~")
	}
	return userSegmentRE.ReplaceAllString(s, "${1}/<user>")
}

// maskARN replaces every AWS ARN with "arn:***".
func maskARN(s string) string {
	return arnRE.ReplaceAllString(s, "arn:"+shareMask)
}

// maskAccountID replaces bare 12-digit AWS account ids with ***. Run AFTER
// maskARN so account ids embedded in an ARN are already gone and only naked
// ids (e.g. `--account 123456789012`) remain to catch.
func maskAccountID(s string) string {
	return accountIDRE.ReplaceAllString(s, shareMask)
}

// RedactForShare returns a copy of r with share-unsafe data removed from the
// free-text fields (Reason, CWD, Command). Structured fields (Decision, Mode,
// Tool, Verb, Subverb, AgentType, SessionID, Pane) are categorical and carry no
// deployment identity, so they pass through untouched.
func RedactForShare(home string, r Record) Record {
	clean := func(s string) string {
		s = maskARN(s)       // whole ARN (incl. its account id) → arn:***
		s = maskAccountID(s) // any remaining bare account id → ***
		s = anonymizeText(s, home)
		return s
	}
	r.Reason = clean(r.Reason)
	r.CWD = clean(r.CWD)
	r.Command = clean(r.Command)
	return r
}
