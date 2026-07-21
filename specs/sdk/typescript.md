# TypeScript SDK Contract

Canonical interface (identical across all seven languages — see `README.md` in this directory for why this isn't seven independent designs):

```
AIDosClient
  chat.complete(request: ChatRequest) -> ChatResponse
  chat.stream(request: ChatRequest) -> Stream<ChatChunk>
  embeddings.create(request: EmbeddingRequest) -> EmbeddingResponse
  images.generate(request: ImageRequest) -> ImageResponse
  audio.transcribe(request: TranscriptionRequest) -> TranscriptionResponse
  audio.synthesize(request: SynthesisRequest) -> SynthesisResponse
  videos.generate(request: VideoRequest) -> Job
  models.list(filter?: ModelFilter) -> Model[]
  models.findCompatible(modelId: string, requirements: CompatibilityRequirements) -> Model[]
```

## TypeScript idiom

```typescript
interface AIDosClient {
  chat: {
    complete(request: ChatRequest): Promise<ChatResponse>;
    stream(request: ChatRequest): AsyncIterable<ChatChunk>;
  };
  embeddings: { create(request: EmbeddingRequest): Promise<EmbeddingResponse>; };
  images: { generate(request: ImageRequest): Promise<ImageResponse>; };
  audio: {
    transcribe(request: TranscriptionRequest): Promise<TranscriptionResponse>;
    synthesize(request: SynthesisRequest): Promise<SynthesisResponse>;
  };
  videos: { generate(request: VideoRequest): Promise<Job>; };
  models: {
    list(filter?: ModelFilter): Promise<Model[]>;
    findCompatible(modelId: string, requirements: CompatibilityRequirements): Promise<Model[]>;
  };
}
```

- **Errors**: thrown `AIDosError` subclasses, mirroring the `ProviderError` taxonomy (`RateLimitedError`, `AuthError`, `ContextLengthExceededError`, and the rest) one-to-one.
- **Config**: constructor options object — `new AIDosClient({ apiKey, baseUrl? })`.
- **Types**: generated directly from `../schemas/*.schema.json` via `json-schema-to-typescript`, never hand-maintained twice.
