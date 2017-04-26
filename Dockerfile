FROM alpine:3.5
MAINTAINER Johnny Horvi <johnny.horvi@gmail.com>

WORKDIR /app

COPY api .
COPY app-config.yaml .
COPY kubeconfig .

CMD ["./api --kubeconfig ./kubeconfig"]