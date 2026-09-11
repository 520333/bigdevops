cat <<EOF >/etc/cron.d/prometheus_reload
* * * * * root /bin/bash /opt/app/prometheus/reload_prometheus_rule.sh
EOF


cat <<"EOF" >/opt/app/prometheus/reload_prometheus_rule.sh
#!/bin/bash
SERVER=192.168.50.1:8080
HOST=$(ip route get 8.8.8.8 | awk '{print $7}')

RES=`curl http://${SERVER}/noAuth/downloadPrometheusAlertRuleMainConfigYaml?ip=${HOST}`
echo $RES

if [ -z "$RES" ]; then
  echo "downloadPrometheusRuleMainConfigYaml.empty"
  exit 2
fi

echo "$RES" > /opt/app/prometheus/rule_tmp.yml

./promtool check rules rule_tmp.yml
if  [ $? -ne 0 ]; then
  echo "rule failed exit"
  exit 3
fi
echo "$RES" > /opt/app/prometheus/rule.yml
curl -XPOST localhost:9090/-/reload
EOF


chmod +x reload_prometheus_rule.sh
bash -x reload_prometheus_rule.sh