FROM golang:1.18 as build

ARG APPLICATION

WORKDIR /build

COPY ../.. .

RUN go build -o /app ./cmd/${APPLICATION}

ENTRYPOINT ["/app"]
