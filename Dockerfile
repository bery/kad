FROM debian

LABEL maintainer="Tomáš Kukrál"

RUN apt-get update && \
  apt-get install -y curl procps && \
  rm -rf /var/lib/apt/lists/*

COPY build_out/kad /bin/

CMD ["/bin/kad"]
