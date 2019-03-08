FROM alpine:3.9

COPY webproxy.nav.no.cer /usr/local/share/ca-certificates/
RUN  apk add --no-cache ca-certificates
RUN  update-ca-certificates

WORKDIR /app

COPY naisd .

CMD /app/naisd --fasit-url=$fasit_url --cluster-subdomain=$cluster_subdomain --clustername=$clustername --istio-enabled=$istio_enabled --authentication-enabled=$authentication_enabled --logtostderr=true
