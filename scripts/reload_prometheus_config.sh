cat <<EOF >/etc/cron.d/prometheus_reload
* * * * * root /bin/bash /opt/app/prometheus/reload_prometheus_config.sh
EOF


cat <<"EOF" >/opt/app/prometheus/reload_prometheus_config.sh
#!/bin/bash
SERVER=192.168.50.1:8080
HOST=$(ip route get 8.8.8.8 | awk '{print $7}')

RES=`curl http://${SERVER}/noAuth/downloadPrometheusMainConfigYaml?ip=${HOST}`
echo $RES

if [ -z "$RES" ]; then
  echo "downloadPrometheusMainConfigYaml.empty"
else
  echo "$RES" > /opt/app/prometheus/prometheus.yml
  curl -XPOST localhost:9090/-/reload
fi
EOF
chmod +x reload_prometheus_config.sh
bash -x reload_prometheus_config.sh