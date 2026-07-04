cat <<EOF >/etc/cron.d/alertmanager_reload
* * * * * root /bin/bash /opt/app/alertmanager/reload_alertmanager_config.sh
EOF


cat <<"EOF" >/opt/app/alertmanager/reload_alertmanager_config.sh
#!/bin/bash
SERVER=192.168.50.1:8080
HOST=$(ip route get 8.8.8.8 | awk '{print $7}')

RES=`curl http://${SERVER}/noAuth/downloadAlertManagerMainConfigYaml?ip=${HOST}`
echo $RES

if [ -z "$RES" ]; then
  echo "downloadAlertManagerMainConfigYaml.empty"
else
  echo "$RES" > /opt/app/alertmanager/alertmanager.yml
  curl -XPOST localhost:9093/-/reload
fi
EOF

chmod +x reload_alertmanager_config.sh
bash -x reload_alertmanager_config.sh