FROM golang:1.23 as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .


FROM golang:1.23

WORKDIR /app
COPY --from=builder /app .

CMD ["go", "test", "./..."]
