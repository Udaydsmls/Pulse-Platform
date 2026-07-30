FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /app/server .

FROM alpine:3.19

COPY --from=builder /app/server /app/server

# 50053 = gRPC, 9090 = Prometheus metrics
EXPOSE 50053 9090

ENTRYPOINT ["/app/server"]
