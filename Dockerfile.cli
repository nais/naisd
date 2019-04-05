FROM alpine:3.9

COPY webproxy.nav.no.cer /usr/local/share/ca-certificates/
RUN  apk add --no-cache ca-certificates
RUN  update-ca-certificates

WORKDIR /app
COPY nais .
CMD ["--help"]
ENTRYPOINT ["./nais"]
