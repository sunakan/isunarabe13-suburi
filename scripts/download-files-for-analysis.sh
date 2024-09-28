#!/bin/bash
set -eu

#
# INPUT1: tmp/db-servers
# INPUT2: tmp/nginx-servers
# OUTPUT1: tmp/analysis/YYYY-MM-DDTHH:mm:ss+09:00/mysqld.cnf.${server}
# OUTPUT2: tmp/analysis/YYYY-MM-DDTHH:mm:ss+09:00/mysql-error.log.${server}
# OUTPUT3: tmp/analysis/YYYY-MM-DDTHH:mm:ss+09:00/mysql-slow-query.log.${server}
# OUTPUT4: tmp/analysis/YYYY-MM-DDTHH:mm:ss+09:00/nginx.conf.${server}
# OUTPUT5: tmp/analysis/YYYY-MM-DDTHH:mm:ss+09:00/nginx-app.conf.${server}
# OUTPUT6: tmp/analysis/YYYY-MM-DDTHH:mm:ss+09:00/nginx-access.log.${server}
# OUTPUT7: tmp/analysis/YYYY-MM-DDTHH:mm:ss+09:00/nginx-error.log.${server}
# OUTPUT8: tmp/analysis/latest -> tmp/analysis/YYYY-MM-DDTHH:mm:ss+09:00
#
# したいこと1: それぞれをDL
# したいこと2: latestディレクトリのシンボリックリンクを貼りなおす
#

readonly INPUT_FILE_1="tmp/db-servers"
readonly INPUT_FILE_2="tmp/nginx-servers"
readonly INPUT_FILE_3="tmp/dns-servers"
readonly CURRENT_TIME="$(TZ='Asia/Tokyo' date +"%Y-%m-%dT%H:%M:%S%z")"
readonly OUTPUT_DIR_PATH="tmp/analysis/${CURRENT_TIME}"

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

if [ ! -f ${INPUT_FILE_3} ]; then
  echo "${INPUT_FILE_3} がありません。用意してください"
  exit 1
fi

mkdir -p "${OUTPUT_DIR_PATH}"

#
# MySQLとNginxとPowerDNSのログをDL
#
while read server; do
  rsync -az -e ssh ${server}:/etc/mysql/mysql.conf.d/mysqld.cnf ${OUTPUT_DIR_PATH}/mysqld.cnf.${server}
  rsync -az -e ssh --rsync-path="sudo rsync" ${server}:/var/log/mysql/error.log ${OUTPUT_DIR_PATH}/mysql-error.log.${server}
  rsync -az -e ssh --rsync-path="sudo rsync" ${server}:/var/log/mysql/mysql-slow.log ${OUTPUT_DIR_PATH}/mysql-slow.log.${server}
  echo "${server}: MySQLの分析用ファイル群をDLしました"
done < ${INPUT_FILE_1}
while read server; do
  rsync -az -e ssh ${server}:/etc/nginx/nginx.conf ${OUTPUT_DIR_PATH}/nginx.conf.${server}
  rsync -az -e ssh ${server}:/etc/nginx/sites-available/isupipe.conf ${OUTPUT_DIR_PATH}/nginx-app.conf.${server}
  rsync -az -e ssh ${server}:/var/log/nginx/access.log ${OUTPUT_DIR_PATH}/nginx-access.log.${server}
  rsync -az -e ssh ${server}:/var/log/nginx/error.log ${OUTPUT_DIR_PATH}/nginx-error.log.${server}
  echo "${server}: Nginxの分析用ファイル群をDLしました"
done < ${INPUT_FILE_2}
while read server; do
  rsync -az -e ssh ${server}:/etc/systemd/system/pdns.service.d/isudns.conf ${OUTPUT_DIR_PATH}/system-pdns-isudns.conf.${server}
  rsync -az -e ssh ${server}:/var/log/pdns/pdns.log ${OUTPUT_DIR_PATH}/pdns.log.${server}
  rsync -az -e ssh ${server}:/var/log/pdns/pdns-error.log ${OUTPUT_DIR_PATH}/pdns-error.log.${server}
  echo "${server}: PowerDNSの分析用ファイル群をDLしました"
done < ${INPUT_FILE_3}

#
# シンボリックリンクを張り直す
#
readonly LATEST_DIR_PATH="tmp/analysis/latest"
rm -rf ${LATEST_DIR_PATH}
ln -sf $(realpath ${OUTPUT_DIR_PATH}) ${LATEST_DIR_PATH}
echo "DONE: シンボリックリンクを張りなおしました"

echo "分析コマンド例"
echo "alp json --sort sum -r -o count,method,uri,min,avg,max,sum --file tmp/analysis/latest/nginx-access.log.*"
