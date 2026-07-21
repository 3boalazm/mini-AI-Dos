# C# SDK Contract

Canonical interface: see `typescript.md`.

## C# idiom

```csharp
public interface IAIDosClient
{
    Task<ChatResponse> ChatCompleteAsync(ChatRequest request);
    IAsyncEnumerable<ChatChunk> ChatStreamAsync(ChatRequest request);
    Task<EmbeddingResponse> EmbeddingsCreateAsync(EmbeddingRequest request);
    Task<ImageResponse> ImagesGenerateAsync(ImageRequest request);
    Task<Job> VideosGenerateAsync(VideoRequest request);
    Task<IReadOnlyList<Model>> ModelsListAsync(ModelFilter? filter = null);
}
```

- **Errors**: thrown `AIDosException` hierarchy, mirroring `ProviderError`.
- **Config**: constructor + `IOptions<AIDosClientOptions>`, following ASP.NET Core's standard options pattern rather than a bespoke builder.
- **DI**: registered via `services.AddAIDosClient(...)`, consistent with how the rest of a typical .NET host wires dependencies — the one language binding where the platform's own idiom for configuration injection is followed rather than a library-specific pattern.
