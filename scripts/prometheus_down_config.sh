cat <<EOF >/etc/cron.d/prometheus_reload
* * * * * root /bin/bash /opt/app/prometheus/prometheus_reload.sh
EOF


cat <<"EOF" >/opt/app/prometheus/prometheus_reload.sh
#!/bin/bash
SERVER=192.168.50.1:8080
HOST=$(ip route get 8.8.8.8 | awk '{print $7}')

RES=`curl http://${SERVER}/noAuth/downloadPrometheusMainConfigYaml?ip=${HOST}`
echo $RES

if test -z $RES;then
  echo "downloadPrometheusMainConfigYaml.empty"
else
  echo "$RES" > /opt/app/prometheus/prometheus.yml
  curl -XPOST localhost:9090/-/reload
fi
EOF
chmod +x prometheus_reload.sh
bash -x prometheus_reload.sh