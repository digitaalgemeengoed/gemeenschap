# dg-beheer: één binary, zonder afhankelijkheden van buiten.
FROM golang:1.24-alpine AS bouw
WORKDIR /src
COPY beheer/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /dg-beheer .

FROM alpine:3.22
RUN adduser -D -H -u 10001 beheer && install -d -o beheer -g beheer -m 750 /data
COPY --from=bouw /dg-beheer /usr/local/bin/dg-beheer
USER beheer
EXPOSE 8080
ENTRYPOINT ["dg-beheer"]
