# Mini AI-DOS gateway — multi-stage build, small non-root runtime.
# No secrets are baked in: all configuration is environment variables
# supplied at run time (see .env.example).

FROM golang:1.25-alpine AS build
WORKDIR /src
# Both modules are needed: gateway's go.mod replaces the foundation
# dependency with the sibling directory. Module downloads (pgx) are
# cached in a separate layer keyed on the go.mod/go.sum pair.
COPY services/foundation/go.mod services/foundation/
COPY services/gateway/go.mod services/gateway/go.sum services/gateway/
WORKDIR /src/services/gateway
RUN go mod download
WORKDIR /src
COPY services/foundation/ services/foundation/
COPY services/gateway/ services/gateway/
WORKDIR /src/services/gateway
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mini-ai-dos ./cmd/gateway

FROM alpine:3.20
RUN adduser -D -u 10001 miniaidos
USER miniaidos
COPY --from=build /out/mini-ai-dos /usr/local/bin/mini-ai-dos
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/health || exit 1
ENTRYPOINT ["mini-ai-dos"]
