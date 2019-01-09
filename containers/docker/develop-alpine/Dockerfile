FROM alpine:3.7

RUN \
  apk add --update go git make gcc musl-dev linux-headers ca-certificates && \
  git clone --depth 1 https://github.com/ETSC3259/etsc && \
  (cd go-etsc && make getsc) && \
  cp go-etsc/build/bin/getsc /getsc && \
  apk del go git make gcc musl-dev linux-headers && \
  rm -rf /go-etsc && rm -rf /var/cache/apk/*

EXPOSE 8545
EXPOSE 30303

ENTRYPOINT ["/getsc"]
