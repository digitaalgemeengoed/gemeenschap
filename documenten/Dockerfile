# dg-documenten: één binary, zonder afhankelijkheden van buiten (vendor/).
FROM golang:1.24-alpine AS bouw
WORKDIR /src
COPY documenten/ ./
RUN CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags="-s -w" -o /dg-documenten .

FROM alpine:3.22
RUN adduser -D -H -u 10001 documenten
COPY --from=bouw /dg-documenten /usr/local/bin/dg-documenten
USER documenten
EXPOSE 8080
ENTRYPOINT ["dg-documenten"]
