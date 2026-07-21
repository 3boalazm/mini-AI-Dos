# Swift SDK Contract

Canonical interface: see `typescript.md`.

## Swift idiom

```swift
protocol AIDosClient {
    func chatComplete(_ request: ChatRequest) async throws -> ChatResponse
    func chatStream(_ request: ChatRequest) -> AsyncThrowingStream<ChatChunk, Error>
    func embeddingsCreate(_ request: EmbeddingRequest) async throws -> EmbeddingResponse
    func imagesGenerate(_ request: ImageRequest) async throws -> ImageResponse
    func videosGenerate(_ request: VideoRequest) async throws -> Job
    func modelsList(filter: ModelFilter?) async throws -> [Model]
}
```

- **Errors**: thrown `AIDosError` enum, one case per `ProviderError` subclass, matched via `catch let error as AIDosError.rateLimited(retryAfter)`.
- **Config**: initializer with named parameters — `AIDosClient(apiKey: ..., baseURL: ...)`.
- **Codable**: request/response types generated to conform to `Codable` directly from `../schemas/*.schema.json`, keeping wire format and Swift model in lockstep without a hand-written mapping layer.
