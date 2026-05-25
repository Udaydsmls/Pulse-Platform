FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /app/server ./cmd/server

FROM alpine:3.19

COPY --from=builder /app/server /app/server

EXPOSE 50051

ENTRYPOINT ["/app/server"]
