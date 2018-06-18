FROM scratch

COPY build_out/kad /bin/

CMD /bin/kad
