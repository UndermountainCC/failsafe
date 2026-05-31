// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package auditlog

import (
	"strings"
	"testing"
)

// TestRedactForShare_NoRawSensitiveData is the HARD GATE. A record
// carrying $HOME paths, a username, an ARN, and a bare account id across all
// three free-text fields (Reason, CWD, Command) must, after share redaction,
// contain ZERO of those raw substrings. This is the contract that makes
// `report --share` safe to paste into a chat or issue.
func TestRedactForShare_NoRawSensitiveData(t *testing.T) {
	home := "/Users/you"
	rec := Record{
		Decision: "block",
		Reason:   "would touch /Users/you/Code/infra via arn:aws:eks:us-east-1:123456789012:cluster/prod",
		CWD:      "/Users/you/Code/infra",
		Command:  "kubectl --context arn:aws:eks:us-east-1:123456789012:cluster/prod delete -f /Users/you/x.yaml; aws sts get-caller-identity --account 123456789012",
	}
	got := RedactForShare(home, rec)
	blob := strings.Join([]string{got.Reason, got.CWD, got.Command}, "\n")
	for _, raw := range []string{
		"/Users/you",            // $HOME path
		"you",                   // username
		"123456789012",          // account id (bare and in-ARN)
		"arn:aws:eks:us-east-1", // ARN body
	} {
		if strings.Contains(blob, raw) {
			t.Errorf("share output leaked %q:\n%s", raw, blob)
		}
	}
}

// TestRedactForShare_CleanRecordUnchanged: a record with nothing sensitive
// passes through byte-for-byte. Over-redaction is safe, but needless mangling
// of benign content erodes trust in the report.
func TestRedactForShare_CleanRecordUnchanged(t *testing.T) {
	home := "/Users/you"
	rec := Record{
		Decision: "allow",
		Reason:   "",
		CWD:      "/opt/work/repo",
		Command:  "kubectl get pods -n default",
	}
	got := RedactForShare(home, rec)
	if got.CWD != rec.CWD || got.Command != rec.Command || got.Reason != rec.Reason {
		t.Errorf("clean record was altered:\n in: %+v\nout: %+v", rec, got)
	}
}

func TestAnonymizeText(t *testing.T) {
	home := "/Users/you"
	cases := []struct{ name, in, want string }{
		{"home prefix to tilde", "/Users/you/Code/infra", "~/Code/infra"},
		{"home embedded in command", "kubectl apply -f /Users/you/x.yaml", "kubectl apply -f ~/x.yaml"},
		{"other user stripped", "/Users/coworker/secrets", "/Users/<user>/secrets"},
		{"linux home stripped", "/home/alice/deploy", "/home/<user>/deploy"},
		{"non-home absolute untouched", "/opt/work/repo", "/opt/work/repo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := anonymizeText(tc.in, home); got != tc.want {
				t.Errorf("anonymizeText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMaskARN(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"eks arn", "arn:aws:eks:us-east-1:123456789012:cluster/prod", "arn:***"},
		{"arn in command", "kubectl --context arn:aws:iam::123456789012:role/admin get", "kubectl --context arn:*** get"},
		{"no arn untouched", "kubectl get pods", "kubectl get pods"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := maskARN(tc.in); got != tc.want {
				t.Errorf("maskARN(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMaskAccountID(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"bare 12-digit masked", "--account 123456789012", "--account ***"},
		{"11-digit untouched", "id 12345678901", "id 12345678901"},
		{"13-digit untouched", "id 1234567890123", "id 1234567890123"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := maskAccountID(tc.in); got != tc.want {
				t.Errorf("maskAccountID(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
