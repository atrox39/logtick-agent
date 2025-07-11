FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o agent -ldflags="-s -w" ./main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/agent .

COPY web ./web

EXPOSE 9090

CMD ["./agent"]

HEALTHCHECK --interval=30s --timeout=10s --retries=3 CMD wget -q -O /dev/null http://localhost:9090/metrics || exit 1
