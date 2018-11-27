FROM debian

LABEL maintainer="tomkukral"

RUN apt-get update && \
  apt-get install -y curl procps && \
  rm -rf /var/lib/apt/lists/*

COPY build_out/kad /bin/

CMD ["/bin/kad"]
