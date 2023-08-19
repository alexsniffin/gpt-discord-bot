# Stage 1: Build Go binary
FROM golang:alpine as builder

WORKDIR /app

COPY . .

RUN go mod download
RUN GOARCH=arm64 go build -o bot .

# Stage 2: Runtime image
FROM alpine:latest

COPY --from=builder /app/bot /app/
WORKDIR /app

CMD ["./bot"]
