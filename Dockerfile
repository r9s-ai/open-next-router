FROM golang:1.24-alpine AS build

WORKDIR /app

COPY . .

RUN go build -o /usr/local/bin/open-next-router ./cmd/open-next-router

FROM alpine:3.20
RUN adduser -D -H -u 10001 app
USER app

WORKDIR /home/app
COPY --from=build /usr/local/bin/open-next-router /usr/local/bin/open-next-router

EXPOSE 3000
ENTRYPOINT ["/usr/local/bin/open-next-router"]
CMD ["--config", "/etc/open-next-router/open-next-router.yaml"]
