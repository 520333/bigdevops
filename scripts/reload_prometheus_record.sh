cat <<EOF >/etc/cron.d/prometheus_reload
* * * * * root /bin/bash /opt/app/prometheus/reload_prometheus_record.sh
EOF


cat <<"EOF" >/opt/app/prometheus/reload_prometheus_record.sh
#!/bin/bash
SERVER=192.168.50.1:8080
HOST=$(ip route get 8.8.8.8 | awk '{print $7}')

RES=`curl http://${SERVER}/noAuth/downloadPrometheusRecordRuleMainConfigYaml?ip=${HOST}`
echo $RES

if [ -z "$RES" ]; then
  echo "downloadPrometheusRecordRuleMainConfigYaml.empty"
  exit 2
fi

echo "$RES" > /opt/app/prometheus/record_tmp.yml

./promtool check rules record_tmp.yml
if  [ $? -ne 0 ]; then
  echo "record failed exit"
  exit 3
fi
echo "$RES" > /opt/app/prometheus/record.yml
curl -XPOST localhost:9090/-/reload
EOF


chmod +x reload_prometheus_record.sh
bash -x reload_prometheus_record.sh