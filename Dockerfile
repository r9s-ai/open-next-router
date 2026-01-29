FROM golang:1.24-alpine AS build

WORKDIR /app

COPY . .

RUN go build -o /usr/local/bin/onr ./cmd/onr

FROM alpine:3.20
RUN adduser -D -H -u 10001 app
USER app

WORKDIR /home/app
COPY --from=build /usr/local/bin/onr /usr/local/bin/onr

EXPOSE 3000
ENTRYPOINT ["/usr/local/bin/onr"]
CMD ["--config", "/etc/onr/onr.yaml"]
