FROM alpine:3.5
MAINTAINER Johnny Horvi <johnny.horvi@nav.no>

RUN apk add --no-cache ca-certificates && \
	update-ca-certificates

WORKDIR /app

COPY naisd .
COPY app-config.yaml .

CMD /app/naisd --fasiturl $fasit_url --logtostderr=true
