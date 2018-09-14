FROM alpine:3.8
MAINTAINER Johnny Horvi <johnny.horvi@nav.no>

COPY webproxy.nav.no.cer /usr/local/share/ca-certificates/
RUN  apk add --no-cache ca-certificates
RUN  update-ca-certificates

WORKDIR /app

COPY naisd .

CMD /app/naisd --fasit-url=$fasit_url --cluster-subdomain=$cluster_subdomain --clustername=$clustername --istio-enabled=$istio_enabled --logtostderr=true
