FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o meso-api . \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o meso-seed ./cmd/seed

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/meso-api .
# The seed one-shot ships alongside the server so lookups/catalog data can be
# loaded in any environment: `docker compose run --rm --entrypoint ./meso-seed api`.
COPY --from=builder /build/meso-seed .
COPY --from=builder /build/migrations ./migrations
EXPOSE 8088
CMD ["./meso-api"]
