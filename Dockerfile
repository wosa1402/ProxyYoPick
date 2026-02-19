# -- build stage --
FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /proxyyopick .

# -- runtime stage --
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /proxyyopick /usr/local/bin/proxyyopick

EXPOSE 8080

ENTRYPOINT ["proxyyopick"]
CMD ["web"]
