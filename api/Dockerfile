FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o meso-api . \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o meso-seed ./cmd/seed

FROM alpine:3.22
# The server reads its migrations and writes nothing, so it runs as a user that
# owns neither.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 meso
WORKDIR /app
COPY --from=builder /build/meso-api .
# The seed one-shot ships alongside the server so lookups/catalog data can be
# loaded in any environment: `docker compose run --rm --entrypoint ./meso-seed api`.
COPY --from=builder /build/meso-seed .
COPY --from=builder /build/migrations ./migrations
USER 10001:10001
EXPOSE 8088
CMD ["./meso-api"]
