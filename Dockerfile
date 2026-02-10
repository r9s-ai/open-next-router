FROM golang:1.24-alpine AS build

WORKDIR /app

COPY . .

RUN go build -o /usr/local/bin/onr ./cmd/onr

FROM alpine:3.20

WORKDIR /root
COPY --from=build /usr/local/bin/onr /usr/local/bin/onr

EXPOSE 3300
ENTRYPOINT ["/usr/local/bin/onr"]
CMD ["--config", "/etc/onr/onr.yaml"]
