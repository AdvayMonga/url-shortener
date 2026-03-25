FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -o url-shortener .



FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/url-shortener .
COPY --from=builder /app/index.html .

EXPOSE 8080

CMD ["./url-shortener"]
