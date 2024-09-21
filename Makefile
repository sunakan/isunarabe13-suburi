.PHONY: build-cfn
build-cfn: cloudformation.yml ## CFnファイルをbuild
	@([ -e tmp/cloudformation.yml ] && echo 'build済みです') || (echo 'buildします' && bash scripts/build-cfn.sh)
	@diff -ur cloudformation.yml tmp/cloudformation.yml | delta

.PHONY: check-authorized-keys
check-authorized-keys: cloudformation.yml ## ISUNARABEに登録する時のSSHの公開鍵
	$(eval SETUP_TOKEN := $(shell cat cloudformation.yml | rq -yJ | jq -r '.Parameters.SetupToken.Default'))
	@curl -s -H "Authorization: Bearer ${SETUP_TOKEN}" "https://api.isunarabe.org/api/setup/authorized_keys"

.PHONY: check-ssh
check-ssh: tmp/ips ## CFnでEC2を設置して、sshできるか確認する
	@cat tmp/ips | xargs -I{} bash -c 'echo "----[ isucon@{} ]" && ssh isucon@{} -i ${SSH_KEY_PATH} "ls"'

.PHONY: show-hosts
show-hosts: tmp/ips ## /etc/hostsに追加する記述をshow
	@head -n1 tmp/ips | xargs -I{} echo '{} pipe.t.isucon.pw'
	@cat tmp/ips | grep -v '#' | nl | while read n ip; do \
	  echo "$${ip} test00$${n}.t.isucon.pw"; \
	done

.PHONY: replace-pem
replace-pem: tmp/ips ## 証明書をreplaceして、Nginxを再起動
	@cat tmp/ips | grep -v '#' | xargs -I{} ssh isucon@{} -i ${SSH_KEY_PATH} "sudo wget -O /etc/nginx/tls/_.t.isucon.pw.crt ${FULLCHAIN_PEM_URL} && sudo wget -O /etc/nginx/tls/_.t.isucon.pw.key ${KEY_PEM_URL}"
	@cat tmp/ips | grep -v '#' | xargs -I{} bash -c 'echo "----[ isucon@{}のNginxを再起動 ]" && ssh isucon@{} -i ${SSH_KEY_PATH} "sudo systemctl reload nginx"'

################################################################################
# エラー文言
################################################################################
cloudformation.yml:
	@echo 'ISUNARABEからcloudformation.ymlをDLしてください' >&2
	exit 1

tmp/ips:
	@echo 'tmp/ipsを記述してください(benchは先頭に#付きで)' >&2
	exit 1

################################################################################
# Utility-Command help
################################################################################
.DEFAULT_GOAL := help

################################################################################
# マクロ
################################################################################
# Makefileの中身を抽出してhelpとして1行で出す
# $(1): Makefile名
# 使い方例: $(call help,{included-makefile})
define help
  grep -E '^[\.a-zA-Z0-9_-]+:.*?## .*$$' $(1) \
  | grep --invert-match "## non-help" \
  | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
endef

################################################################################
# タスク
################################################################################
.PHONY: help
help: ## Make タスク一覧
	@echo '######################################################################'
	@echo '# Makeタスク一覧'
	@echo '# $$ make XXX'
	@echo '# or'
	@echo '# $$ make XXX --dry-run'
	@echo '######################################################################'
	@echo $(MAKEFILE_LIST) \
	| tr ' ' '\n' \
	| xargs -I {included-makefile} $(call help,{included-makefile})
