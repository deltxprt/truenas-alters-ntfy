# syntax=docker/dockerfile:1

FROM golang:1.20-alpine as build

WORKDIR /app

COPY go.mod ./

RUN go mod download

COPY . .

run go build -o truenas-alerts-ntfy

FROM golang:1.20-alpine

WORKDIR /

COPY --from=build /app/truenas-alerts-ntfy /truenas-alerts-ntfy

CMD [ "/truenas-alerts-ntfy" ]

