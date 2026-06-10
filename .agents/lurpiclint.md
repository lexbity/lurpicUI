# Agent instructions for lurpicUI development
#
# When working on this codebase, run lurpiclint before committing:
#
#   go run ./cmd/lurpiclint check ./...
#
# This is the build gate: non-zero exit means the code has violations.
# The gate checks:
#   LL001 (warn)   — raw LayoutRole literal (prefer composition)
#   LL002 (warn)   — absolute coordinate placement (prefer relative layout)
#   LL003 (error)  — hand-rolled layout container (use built-in marks)
#   LL004 (info)   — shape-match suggestion (consider using an existing mark)
#   LL010 (error)  — facet/projection may not import render
#   LL011 (error)  — no goroutines or raw channels in facet code
#   LL012 (warn)   — domain state in facet field (keep facets stateless)
#   LL013 (warn)   — theme token captured in OnAttach (resolve at projection)
#   LL014 (error)  — overlay missing layer/hit/dismissal contracts
#   LL015 (error)  — stable claim without verified evidence
#
# To see the capability index:
#
#   go run ./cmd/lurpiclint capabilities
#
# To get help for a specific rule:
#
#   go run ./cmd/lurpiclint explain <rule-id>
#
# Verified = lint-clean + conformance-green.
