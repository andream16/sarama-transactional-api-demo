FROM golang:1.19 as build

ARG APPLICATION

WORKDIR /build

COPY ../.. .

RUN go build -o /app ./cmd/${APPLICATION}

ENTRYPOINT ["/app"]
