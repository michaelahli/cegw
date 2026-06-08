FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o cegw ./cmd/cegw

FROM scratch

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cegw /cegw
COPY --from=builder /build/docs /docs
COPY --from=builder /build/pkg/client/example /pkg/client/example

RUN mkdir /tmp

EXPOSE 50051 8080

ENTRYPOINT ["/cegw"]
