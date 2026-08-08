module github.com/ai-dos/gateway

go 1.22

require github.com/ai-dos/foundation v0.0.0

// The foundation module lives in this repository; the replace directive
// (not only go.work) means this module also builds outside the
// workspace — inside the Docker build, in CI checkouts, anywhere.
replace github.com/ai-dos/foundation => ../foundation
