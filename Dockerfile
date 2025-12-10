FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o scrapfly-mcp ./cmd/scrapfly-mcp

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/scrapfly-mcp .

EXPOSE 8080

ENV SCRAPFLY_API_KEY="your-api-key"

CMD ["./scrapfly-mcp", "-http", ":8080"]

