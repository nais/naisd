FROM alpine:3.5
MAINTAINER Johnny Horvi <johnny.horvi@nav.no>

COPY webproxy.crt /usr/local/share/ca-certificates/
RUN apk add --no-cache ca-certificates
RUN	update-ca-certificates

WORKDIR /app

COPY naisd .

CMD /app/naisd --fasit-url=$fasit_url --cluster-subdomain=$cluster_subdomain --logtostderr=true
