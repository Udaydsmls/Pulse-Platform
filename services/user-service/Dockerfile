FROM golang:1.22-alpine AS builder

WORKDIR /build
COPY . .
RUN go mod tidy && go build -o /server .

FROM alpine:3.19
COPY --from=builder /server /server
EXPOSE 8081
ENTRYPOINT ["/server"]
