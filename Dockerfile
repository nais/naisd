FROM alpine:3.5
MAINTAINER Johnny Horvi <johnny.horvi@nav.no>

RUN apk add --no-cache ca-certificates && \
	update-ca-certificates

WORKDIR /app

COPY naisd .

CMD /app/naisd --fasit-url=$fasit_url --cluster-subdomain=$cluster_subdomain --logtostderr=true
