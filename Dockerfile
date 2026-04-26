FROM golang:1.26.0-alpine AS builder

WORKDIR /build

ADD go.mod ./ 

COPY . . 


RUN go build -o app ./cmd/main.go

FROM alpine

COPY --from=builder /build/app /app

CMD ["./app"]