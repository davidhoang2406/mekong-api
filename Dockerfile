FROM golang:1.25-alpine AS builder
WORKDIR /app
RUN apk add --no-cache gcc g++ musl-dev
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /mekong-api ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /mekong-api /usr/local/bin/mekong-api
COPY --from=builder /app/migrations /migrations
USER nobody
EXPOSE 8090
HEALTHCHECK --interval=15s --timeout=5s CMD wget -qO- http://localhost:8090/api/v1/health || exit 1
CMD ["mekong-api"]
