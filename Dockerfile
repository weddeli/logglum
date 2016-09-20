FROM alpine:3.4

RUN mkdir -p /opt/fwd/ && adduser -S loggly 
RUN apk add --no-cache ca-certificates
WORKDIR /opt/fwd/

ENV PATH /opt/fwd/:$PATH

USER loggly

COPY  logglum /opt/fwd/

CMD ["./logglum"]
