#!/bin/bash
set -eu

#
# INPUT: tmp/webapp-servers
# OUTPUT1: tmp/webapp-go/YYYY-MM-DDTHH:mm:ssZ/main.go.before
# OUTPUT2: tmp/webapp-go/YYYY-MM-DDTHH:mm:ssZ/main.go.after
# OUTPUT3: tmp/webapp-go/YYYY-MM-DDTHH:mm:ssZ/diff
#
# したいこと1: conf.InterpolateParams = true を追記
# したいこと2: diffをとっておく
# したいこと3: 配布
# したいこと4: make build
# したいこと5: アプリ再起動

readonly INPUT_FILE="tmp/webapp-servers"
readonly CURRENT_TIME="$(TZ='Asia/Tokyo' date +"%Y-%m-%dT%H:%M:%S%z")"
readonly OUTPUT_DIR_PATH="tmp/webapp-go/${CURRENT_TIME}"
readonly OUTPUT_FILE_1="${OUTPUT_DIR_PATH}/main.go.before"
readonly OUTPUT_FILE_2="${OUTPUT_DIR_PATH}/main.go.after"
if [ ! -f ${INPUT_FILE} ]; then
  echo "${INPUT_FILE} がありません。用意してください"
  exit 1
fi

mkdir -p "${OUTPUT_DIR_PATH}"

readonly FIRST_SERVER=$(head -n1 ${INPUT_FILE})
scp ${FIRST_SERVER}:/home/isucon/webapp/go/main.go ${OUTPUT_FILE_1}
cp ${OUTPUT_FILE_1} ${OUTPUT_FILE_2}

sed -i '' '/conf.InterpolateParams/d' ${OUTPUT_FILE_2}
awk '
  /conf\.ParseTime/ && !found {
    print
    print "\tconf.InterpolateParams = true"
    found = 1
    next
  }
  {print}' ${OUTPUT_FILE_2} > tmp/temp-main.go
mv tmp/temp-main.go ${OUTPUT_FILE_2}

set +e
diff -u ${OUTPUT_FILE_1} ${OUTPUT_FILE_2} > ${OUTPUT_DIR_PATH}/diff
set -e

readonly LATEST_DIR_PATH="tmp/webapp-go/latest"
rm -rf ${LATEST_DIR_PATH} && ln -sf $(realpath ${OUTPUT_DIR_PATH}) ${LATEST_DIR_PATH}

# 各サーバーへ配布して再起動
# ssh -n をしている理由: 標準入力が占有されてしまい1回分しか回らなくなる。それを回避するため
while read server; do
  scp ${OUTPUT_FILE_2} ${server}:/home/isucon/webapp/go/main.go
  ssh -n ${server} "export PATH=\$PATH:/home/isucon/local/golang/bin && cd /home/isucon/webapp/go && make build && sudo systemctl restart isupipe-go"
done < ${INPUT_FILE}
