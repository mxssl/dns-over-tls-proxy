FROM golang:1.11.5-alpine3.9 as builder

WORKDIR /go/src/github.com/mxssl/dns-over-tls-proxy
COPY . .

# Compile the app
ENV GO111MODULE=on
RUN apk add --no-cache ca-certificates git
RUN CGO_ENABLED=0 \
  GOOS=`go env GOHOSTOS` \
  GOARCH=`go env GOHOSTARCH` \
  go build -o app

# Copy compiled binary to clear Alpine Linux image
FROM alpine:3.9
WORKDIR /
RUN apk add --no-cache ca-certificates
COPY --from=builder /go/src/github.com/mxssl/dns-over-tls-proxy .
RUN chmod +x app
CMD ["./app"]
