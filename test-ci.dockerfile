FROM golang:1.22 as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .


FROM golang:1.22

WORKDIR /app
COPY --from=builder /app .

CMD ["go", "test", "./..."]
