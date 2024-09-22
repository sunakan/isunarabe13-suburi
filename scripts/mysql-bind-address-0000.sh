#!/bin/bash
set -eu

#
# INPUT: tmp/db-servers
# OUTPUT1: tmp/mysql/YYYY-MM-DDTHH:mm:ssZ/mysqld.cnf.before
# OUTPUT2: tmp/mysql/YYYY-MM-DDTHH:mm:ssZ/mysqld.cnf.after
# OUTPUT3: tmp/mysql/YYYY-MM-DDTHH:mm:ssZ/diff
#
# したいこと1: 指定した秒数以上かかったクエリをスロークエリとしてログを出すconfを作成
# したいこと2: 作成したconfを配布
# したいこと3: 作成したconfを配布
# したいこと4: diffをとっておく
# したいこと5: latestディレクトリのシンボリックリンクを貼りなおす
#

readonly INPUT_FILE="tmp/db-servers"
readonly CURRENT_TIME="$(TZ='Asia/Tokyo' date +"%Y-%m-%dT%H:%M:%S%z")"
readonly OUTPUT_DIR_PATH="tmp/mysql/${CURRENT_TIME}"
readonly OUTPUT_FILE_1="${OUTPUT_DIR_PATH}/mysqld.cnf.before"
readonly OUTPUT_FILE_2="${OUTPUT_DIR_PATH}/mysqld.cnf.after"

if [ ! -f ${INPUT_FILE} ]; then
  echo "${INPUT_FILE} がありません。用意してください"
  exit 1
fi

mkdir -p "${OUTPUT_DIR_PATH}"

readonly FIRST_SERVER=$(head -n1 ${INPUT_FILE})
scp ${FIRST_SERVER}:/etc/mysql/mysql.conf.d/mysqld.cnf ${OUTPUT_FILE_1}
cp ${OUTPUT_FILE_1} ${OUTPUT_FILE_2}

sed -i '' '/^bind-address/d' ${OUTPUT_FILE_2}
echo 'bind-address = 0.0.0.0' >> ${OUTPUT_FILE_2}

set +e
diff -u ${OUTPUT_FILE_1} ${OUTPUT_FILE_2} > ${OUTPUT_DIR_PATH}/diff
set -e

readonly LATEST_DIR_PATH="tmp/mysql/latest"
rm -rf ${LATEST_DIR_PATH} && ln -sf $(realpath ${OUTPUT_DIR_PATH}) ${LATEST_DIR_PATH}

# 各サーバーへ配布して再起動
# ssh -n をしている理由: 標準入力が占有されてしまい1回分しか回らなくなる。それを回避するため
while read server; do
  ssh -n ${server} "sudo rm -rf /tmp/mysqld.cnf"
  scp ${OUTPUT_FILE_2} ${server}:/tmp/mysqld.cnf
  ssh -n ${server} "sudo chown root:root /tmp/mysqld.cnf && sudo chmod 644 /tmp/mysqld.cnf"
  ssh -n ${server} "sudo mv /tmp/mysqld.cnf /etc/mysql/mysql.conf.d/mysqld.cnf"
  echo "${server}: setup mysqld.cnf & restart"
done < ${INPUT_FILE}
