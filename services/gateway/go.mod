module github.com/ai-dos/gateway

go 1.25.0

require (
	github.com/ai-dos/foundation v0.0.0
	github.com/jackc/pgx/v5 v5.10.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)

// The foundation module lives in this repository; the replace directive
// (not only go.work) means this module also builds outside the
// workspace — inside the Docker build, in CI checkouts, anywhere.
replace github.com/ai-dos/foundation => ../foundation
