# syntax=docker/dockerfile:1
FROM golang:1.22 AS builder
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download && go build -o /out/server ./cmd/server

FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=builder /out/server /server
ENV PORT=8080
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/server"]


