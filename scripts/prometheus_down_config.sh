#!/bin/bash
SERVER=192.168.50.1:8080
HOST=$(ip route get 8.8.8.8 | awk '{print $7}')

curl http://${SERVER}/noAuth/downloadPrometheusMainConfigYaml?ip=${HOST} >/opt/app/prometheus/prometheus.yml

curl -XPOST localhost:9090/-/reload