FROM golang:1.24.2 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o email-service .

FROM alpine:3.21

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/email-service ./email-service

RUN chmod +x email-service

EXPOSE 3000 50051

CMD ["./email-service"]
