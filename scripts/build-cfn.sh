#!/bin/bash
set -eu

# Check using commands
required_commands=("jq" "rq" "envsubst" "curl")
missing_commands=()
for cmd in "${required_commands[@]}"; do
  if ! command -v "$cmd" &> /dev/null; then
    missing_commands+=("$cmd")
  fi
done
if [ ${#missing_commands[@]} -ne 0 ]; then
  echo "以下のコマンドが見つかりません。インストールしてください:"
  for cmd in "${missing_commands[@]}"; do
    echo "- $cmd"
  done
  exit 1
fi


#
# ISUNARABEのcloudformation.ymlを修正したCFnファイルをビルド
#
# INPUT1: cloudformation.template.yml
# INPUT2: cloudformation.yml # ISUNARABEからDLしたCFnファイル
# OUTPUT: tmp/cloudformation.yml
#
# したいこと: 22,443番ポートをMyIPからのみ許可する
#
export ENV_MY_IP="$(curl -s https://ipinfo.io/ip)/32"
export ENV_SETUP_TOKEN="$(cat cloudformation.yml | rq -yJ | jq -r '.Parameters.SetupToken.Default')"
envsubst '$ENV_MY_IP $ENV_SETUP_TOKEN' < cloudformation.template.yml > tmp/cloudformation.yml
