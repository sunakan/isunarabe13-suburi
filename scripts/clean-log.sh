#!/bin/bash
set -eu

#
# INPUT1: tmp/db-servers
# INPUT2: tmp/nginx-servers
# INPUT3: tmp/dns-servers
#
# したいこと1: MySQLのログをリセットして再起動
# したいこと2: Nginxのログをリセットして再起動
# したいこと3: PowerDNSのログをリセットして再起動
#

readonly INPUT_FILE_1="tmp/db-servers"
readonly INPUT_FILE_2="tmp/nginx-servers"
# readonly INPUT_FILE_3="tmp/dns-servers"

#
# バリデーション
#
if [ ! -f ${INPUT_FILE_1} ]; then
  echo "${INPUT_FILE_1} がありません。用意してください"
  exit 1
fi
if [ ! -f ${INPUT_FILE_2} ]; then
  echo "${INPUT_FILE_2} がありません。用意してください"
  exit 1
fi
# if [ ! -f ${INPUT_FILE_3} ]; then
#   echo "${INPUT_FILE_3} がありません。用意してください"
#   exit 1
# fi

#
# MySQLとNginxをリセットして再起動
#
while read server; do
  ssh -n ${server} "sudo -u mysql mv /var/log/mysql/error.log /var/log/mysql/error.log.old && sudo -u mysql mv /var/log/mysql/mysql-slow.log /var/log/mysql/mysql-slow.log.old && sudo systemctl restart mysql"
  echo "${server}: MySQLのログをリセットしました"
done < ${INPUT_FILE_1}
while read server; do
  ssh -n ${server} "sudo mv /var/log/nginx/error.log /var/log/nginx/error.log.old && sudo mv /var/log/nginx/access.log /var/log/nginx/access.log.old && sudo systemctl reload nginx"
  echo "${server}: Nginxのログをリセットしました"
done < ${INPUT_FILE_2}

# PowerDNSをリセットして再起動していたが、不要になったため、コメントアウト
# while read server; do
#   ssh -n ${server} "sudo mv /var/log/pdns/pdns.log /var/log/pdns/pdns.log.old && sudo mv /var/log/pdns/pdns-error.log /var/log/pdns/pdns-error.log.old && sudo systemctl restart pdns"
#   echo "${server}: PowerDNSのログをリセットしました"
# done < ${INPUT_FILE_3}
