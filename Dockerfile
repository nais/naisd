FROM alpine:3.5
MAINTAINER Johnny Horvi <johnny.horvi@nav.no>

WORKDIR /app

COPY naisd .
COPY app-config.yaml .

CMD ["/app/naisd", "--logtostderr=true"]
