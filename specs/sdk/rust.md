# Rust SDK Contract

Canonical interface: see `typescript.md`.

## Rust idiom

```rust
#[async_trait]
trait AIDosClient {
    async fn chat_complete(&self, request: ChatRequest) -> Result<ChatResponse, AIDosError>;
    fn chat_stream(&self, request: ChatRequest) -> impl Stream<Item = Result<ChatChunk, AIDosError>>;
    async fn embeddings_create(&self, request: EmbeddingRequest) -> Result<EmbeddingResponse, AIDosError>;
    async fn images_generate(&self, request: ImageRequest) -> Result<ImageResponse, AIDosError>;
    async fn videos_generate(&self, request: VideoRequest) -> Result<Job, AIDosError>;
    async fn models_list(&self, filter: Option<ModelFilter>) -> Result<Vec<Model>, AIDosError>;
}
```

- **Errors**: `Result<T, AIDosError>`, `AIDosError` an enum with one variant per `ProviderError` subclass — matched exhaustively, not caught by inheritance, since Rust doesn't have exceptions.
- **Config**: builder pattern — `AIDosClient::builder().api_key(...).build()`.
- **Async runtime**: `tokio`, matching the ecosystem the Go server's own client libraries are most commonly paired against for this kind of workload.
