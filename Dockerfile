FROM golang:1.27-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./ 
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server .


FROM alpine:latest
WORKDIR /app


COPY --from=builder /build/server .
EXPOSE 3000

CMD ["./server"]


