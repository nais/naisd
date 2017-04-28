FROM alpine:3.5
MAINTAINER Johnny Horvi <johnny.horvi@nav.no>

WORKDIR /app

COPY api .
COPY app-config.yaml .

CMD ["/app/api"]
#CMD ["/app/api", "--kubeconfig", "/app/kubeconfig"]
