FROM golang:1.25-bookworm AS builder
WORKDIR /app
RUN apt-get update && apt-get install -y gcc g++ && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /mekong-api ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates wget && rm -rf /var/lib/apt/lists/*
COPY --from=builder /mekong-api /usr/local/bin/mekong-api
COPY --from=builder /app/migrations /migrations
USER nobody
EXPOSE 8090
HEALTHCHECK --interval=15s --timeout=5s CMD wget -qO- http://localhost:8090/api/v1/health || exit 1
CMD ["mekong-api"]
