FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /gophkeeper-server ./cmd/server

FROM alpine:3.21 AS server

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /gophkeeper-server /app/gophkeeper-server
COPY config/content.yml /app/config/content.yml

EXPOSE 8080

ENTRYPOINT ["/app/gophkeeper-server"]
