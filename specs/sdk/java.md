# Java SDK Contract

Canonical interface: see `typescript.md`.

## Java idiom

```java
public interface AIDosClient {
    CompletableFuture<ChatResponse> chatComplete(ChatRequest request);
    Flow.Publisher<ChatChunk> chatStream(ChatRequest request);
    CompletableFuture<EmbeddingResponse> embeddingsCreate(EmbeddingRequest request);
    CompletableFuture<ImageResponse> imagesGenerate(ImageRequest request);
    CompletableFuture<Job> videosGenerate(VideoRequest request);
    CompletableFuture<List<Model>> modelsList(ModelFilter filter);
}
```

- **Errors**: checked `AIDosException` hierarchy, one subclass per `ProviderError` — deliberately checked, not unchecked, so a caller can't silently ignore a rate-limit or auth failure the way an unchecked exception permits.
- **Config**: builder — `AIDosClient.builder().apiKey(...).build()`.
- **Types**: generated via `jsonschema2pojo` from `../schemas/*.schema.json`.
