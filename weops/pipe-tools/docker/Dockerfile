#
# build container
#
FROM --platform=linux/amd64 golang:1.19-alpine as builder
WORKDIR /

COPY oracledb_exporter /oracledb_exporter
ADD default-metrics.toml /default-metrics.toml

EXPOSE     9161
ENTRYPOINT [ "/oracledb_exporter" ]
