FROM golang:1.13.7 as build

WORKDIR /app

ENV CGO_ENABLED 0
ENV GOOS linux

COPY . .
RUN go build -a -installsuffix cgo -o logglum ./cmd


FROM alpine:3.11

RUN mkdir -p /opt/logglum/ && adduser -S logglum
RUN apk add --no-cache ca-certificates
WORKDIR /opt/logglum/

ENV PATH /opt/logglum/:$PATH

USER logglum

COPY  --from=build  /app/logglum /opt/logglum/

CMD ["./logglum"]
