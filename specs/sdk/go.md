# Go SDK Contract

Canonical interface: see `typescript.md`. Notable here specifically because the Gateway's own hot-path implementation is Go (TDS §1) — this client and the server it talks to share the same `Provider*` type definitions where practical, generated once, not maintained twice.

## Go idiom

```go
type AIDosClient interface {
    ChatComplete(ctx context.Context, req ChatRequest) (ChatResponse, error)
    ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, <-chan error)
    EmbeddingsCreate(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
    ImagesGenerate(ctx context.Context, req ImageRequest) (ImageResponse, error)
    AudioTranscribe(ctx context.Context, req TranscriptionRequest) (TranscriptionResponse, error)
    AudioSynthesize(ctx context.Context, req SynthesisRequest) (SynthesisResponse, error)
    VideosGenerate(ctx context.Context, req VideoRequest) (Job, error)
    ModelsList(ctx context.Context, filter *ModelFilter) ([]Model, error)
    ModelsFindCompatible(ctx context.Context, modelID string, req CompatibilityRequirements) ([]Model, error)
}
```

- **Errors**: returned `error`, typed via `errors.As` against the `ProviderError` taxonomy — `var rateLimitErr *RateLimitedError; errors.As(err, &rateLimitErr)`.
- **Config**: functional options — `NewAIDosClient(WithAPIKey(...), WithTimeout(...))`.
- **Context**: every method takes `context.Context` first, Go convention for cancellation/deadlines — this is the one language where that's a contract requirement, not a style preference, since the server this talks to is written the same way.
