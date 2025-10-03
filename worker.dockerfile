FROM golang:1.25

WORKDIR /app

COPY . .

RUN go build -o main ./cmd/worker

CMD ["./main"]
