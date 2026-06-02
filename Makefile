# Doc validation harness.
.PHONY: validate-docs validate-docs-live
validate-docs:        ## headless doc validation (CI runs this)
	bats test/docs/*.bats
validate-docs-live:   ## GUI-launch checks (local only; needs WezTerm/iTerm)
	bats test/docs/live-gui/*.bats
