FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/web

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/
COPY config/config.yaml /root/config/config.yaml

COPY --from=builder /app/main .

EXPOSE 8080

CMD ["./main"]
