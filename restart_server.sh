#!/bin/sh
ps aux | grep 'kora serve' | grep -v grep | awk '{print $2}' | xargs -r kill -9
sleep 1
cd /home/asena/projects/kora
KORA_DB_TYPE=mysql DB_DSN="root:kora123@tcp(127.0.0.1:3306)/kora_platform?parseTime=true&multiStatements=true" nohup ./kora serve --port 8000 > /tmp/kora_server.log 2>&1 &
sleep 2
echo "restarted"
