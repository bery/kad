FROM debian

RUN apt-get update && apt-get install -y curl && rm /var/lib/apt/lists/*


COPY build_out/kad /bin/

CMD ["/bin/kad"]
