FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,mode=0777,target=/root/.cache/go-build \
    --mount=type=cache,mode=0777,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,mode=0777,target=/root/.cache/go-build \
    --mount=type=cache,mode=0777,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o scrapfly-mcp ./cmd/scrapfly-mcp

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/scrapfly-mcp .

ENV PORT=8080

CMD ["./scrapfly-mcp"]

