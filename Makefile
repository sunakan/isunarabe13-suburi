.PHONY: build-cfn
build-cfn: cloudformation.yml ## CFnファイルをbuild
	@([ -e tmp/cloudformation.yml ] && echo 'build済みです') || (echo 'buildします' && bash scripts/build-cfn.sh)
	@diff -ur cloudformation.yml tmp/cloudformation.yml | delta

.PHONY: ssh-config-for-isucon
ssh-config-for-isucon:
	@mkdir -p ~/.ssh/config-for-isucon.d
	@aws ec2 describe-instances --output json --query 'Reservations[].Instances[]' \
	| jq -rc '.[] | {ip: .NetworkInterfaces[0].Association.PublicIp, name: .Tags[] | select(.Key == "Name") | .Value}' \
	| jq -src '. | sort_by(.name)[] | ["isu-\(.name | split("-")[1])", .ip] | @csv' \
	| sed 's/"//g' \
	| awk -F, '{print "Host "$$1"\n  HostName "$$2"\n  User isucon\n  IdentityFile ~/.ssh/id_rsa\n  StrictHostKeyChecking no"}' > ~/.ssh/config-for-isucon.d/config
	@chmod 644 ~/.ssh/config-for-isucon.d/config

.PHONY: check-authorized-keys
check-authorized-keys: cloudformation.yml ## ISUNARABEに登録する時のSSHの公開鍵
	$(eval SETUP_TOKEN := $(shell cat cloudformation.yml | rq -yJ | jq -r '.Parameters.SetupToken.Default'))
	@curl -s -H "Authorization: Bearer ${SETUP_TOKEN}" "https://api.isunarabe.org/api/setup/authorized_keys"

.PHONY: check-ssh
check-ssh: tmp/servers ## CFnでEC2を設置して、sshできるか確認する
	@cat tmp/servers | xargs -I{} bash -c 'echo "----[ {} ]" && ssh {} "ls"'

.PHONY: show-hosts
show-hosts: tmp/servers ## /etc/hostsに追加する記述をshow
	@grep -A1 'isu-1' ~/.ssh/config-for-isucon.d/config | grep HostName | cut -d' ' -f4 | xargs -I{} echo "{} pipe.t.isucon.pw"
	@grep -A1 -E 'isu-\d' ~/.ssh/config-for-isucon.d/config | grep HostName | cut -d' ' -f4 | nl | while read n ip; do \
	  echo "$${ip} test00$${n}.t.isucon.pw"; \
	done

################################################################################
# 各Hostで入れておきたいツール群
################################################################################
.PHONY: setup-tools
setup-tools: tmp/servers ## 各Hostでツール群をインストール
	@cat tmp/servers | xargs -I{} ssh {} "sudo apt-get update && sudo apt-get install -y psmisc tmux tree make jq neovim git"

################################################################################
# MySQL
################################################################################
.PHONY: enable-mysql-slowquery-log
enable-mysql-slowquery-log: ## MySQLのslowqueryログ等を有効化
	@bash scripts/enable-mysql-slowquery-log.sh

.PHONY: mysql-bind-address-0000
mysql-bind-address-0000: ## MySQLのbind-addressを0.0.0.0にする
	@bash scripts/mysql-bind-address-0000.sh

.PHONY: create-mysql-user
create-mysql-user: tmp/db-servers ## MySQLのユーザーを作成(user: isucon, pass: isucon)
	@cat tmp/db-servers | xargs -I{} ssh {} "sudo mysql -e \"create user if not exists 'isucon'@'%' identified by 'isucon';\""
	@cat tmp/db-servers | xargs -I{} ssh {} "sudo mysql -e \"grant all privileges on isupipe.* to 'isucon'@'%';\""
	@cat tmp/db-servers | xargs -I{} ssh {} "sudo mysql -e \"grant all privileges on isudns.* to 'isucon'@'%';\""
	@cat tmp/db-servers | xargs -I{} ssh {} "sudo systemctl restart mysql"

################################################################################
# Nginx
################################################################################
.PHONY: replace-pem
replace-pem: tmp/nginx-servers ## 証明書をreplaceして、Nginxを再起動
	mkdir -p tmp/nginx-tls/
	gh release view --repo KOBA789/t.isucon.pw --json assets --jq '.assets[] | select(.name == "key.pem" or .name == "fullchain.pem") | .url' | xargs -I{} curl -L --output-dir tmp/nginx-tls/ -O {}
	mv tmp/nginx-tls/key.pem tmp/nginx-tls/_.t.isucon.pw.key
	mv tmp/nginx-tls/fullchain.pem tmp/nginx-tls/_.t.isucon.pw.crt
	@cat tmp/nginx-servers | xargs -I{} rsync -az -e ssh --rsync-path="sudo rsync" tmp/nginx-tls/ {}:/etc/nginx/tls/
	@cat tmp/nginx-servers | xargs -I{} bash -c 'echo "----[ {}:Nginx 再起動 ]" && ssh {} "sudo systemctl reload nginx"'

.PHONY: replace-nginx-conf
replace-nginx-conf: tmp/servers ## Nginxのログのjson化など
	@cat tmp/servers | grep -v 'bench' | xargs -I{} scp nginx/nginx.conf {}:/tmp/nginx.conf
	@cat tmp/servers | grep -v 'bench' | xargs -I{} ssh {} "sudo mv /tmp/nginx.conf /etc/nginx/nginx.conf && sudo chown root:root /etc/nginx/nginx.conf && sudo chmod 644 /etc/nginx/nginx.conf"

.PHONY: clean-nginx-log-and-reload
clean-nginx-log-and-reload: tmp/servers ## Nginxのログを削除して、再起動
	@cat tmp/servers | grep -v 'bench' | xargs -I{} ssh {} "sudo rm -f /var/log/nginx/access.log /var/log/nginx/error.log && sudo systemctl reload nginx"

################################################################################
# アプリ
################################################################################
.PHONY: true-interpolate-params
true-interpolate-params: ## InterpolateParams=trueにして、アプリをビルド&再起動
	@bash scripts/true-interpolate-params.sh

.PHONY: rsync-app-and-build-and-restart
rsync-app-and-build-and-restart: tmp/webapp-servers ## アプリをrsyncしてビルド&再起動
	@make reset-pdns-zone
	@cat tmp/webapp-servers | xargs -I{} rsync -az -e ssh --exclude=".idea" --exclude=".tool-versions" --exclude=".gitignore" ./rsync-webapp-go/  {}:/home/isucon/webapp/go/
	@cat tmp/webapp-servers | xargs -I{} ssh {} "mkdir -p /home/isucon/webapp/public/images"
	@cat tmp/webapp-servers | xargs -I{} ssh {} "export PATH=\$$PATH:/home/isucon/local/golang/bin && cd /home/isucon/webapp/go && make build && sudo systemctl restart isupipe-go"
	@cat tmp/webapp-servers | xargs -I{} rsync -az -e ssh --rsync-path="sudo rsync" ./nginx/rsync-etc-nginx-sites-available-isupipe.conf {}:/etc/nginx/sites-available/isupipe.conf
	@cat tmp/webapp-servers | xargs -I{} ssh {} "sudo chown root:root /etc/nginx/sites-available/isupipe.conf && sudo chmod 644 /etc/nginx/sites-available/isupipe.conf && sudo nginx -t && sudo systemctl reload nginx"
	@make clean-log

.PHONY: replace-ISUCON13_MYSQL_DIALCONFIG_ADDRESS
replace-ISUCON13_MYSQL_DIALCONFIG_ADDRESS: tmp/webapp-servers ## ISUCON13_MYSQL_DIALCONFIG_ADDRESSをDB専用のIPに置換する
	@cat tmp/webapp-servers | xargs -I{} ssh {} "sed -i '/ISUCON13_MYSQL_DIALCONFIG_ADDRESS/d' ~/env.sh && echo 'ISUCON13_MYSQL_DIALCONFIG_ADDRESS=\"192.168.0.13\"' >> ~/env.sh"

################################################################################
# PowerDNS
################################################################################
.PHONY: setup-pdns
setup-pdns: tmp/dns-servers ## PowerDNSのセットアップ
	@cat tmp/dns-servers | grep -v 'bench' | xargs -I{} ssh {} "sudo mkdir -p /var/log/pdns/ && sudo chown -R pdns:pdns /var/log/pdns/"
	@cat tmp/dns-servers | grep -v 'bench' | xargs -I{} rsync -az -e ssh --rsync-path="sudo rsync" pdns/etc/systemd/system/pdns.service.d/isudns.conf {}:/etc/systemd/system/pdns.service.d/isudns.conf
	@cat tmp/dns-servers | grep -v 'bench' | xargs -I{} ssh {} "sudo systemctl daemon-reload && sudo systemctl restart pdns"

.PHONY: rsync-pdns-and-restart
rsync-pdns-and-restart: tmp/dns-servers ## PowerDNSのconfigを更新して再起動
	@cat tmp/dns-servers | grep -v 'bench' | xargs -I{} ssh {} "sudo mkdir -p /var/log/pdns/ && sudo chown -R pdns:pdns /var/log/pdns/"
	@cat tmp/dns-servers | grep -v 'bench' | xargs -I{} rsync -az -e ssh --rsync-path="sudo rsync" pdns/etc/systemd/system/pdns.service.d/isudns.conf {}:/etc/systemd/system/pdns.service.d/isudns.conf
	@cat tmp/dns-servers | grep -v 'bench' | xargs -I{} rsync -az -e ssh --rsync-path="sudo rsync" pdns/etc/powerdns/pdns.conf {}:/etc/powerdns/pdns.conf
	@cat tmp/dns-servers | grep -v 'bench' | xargs -I{} rsync -az -e ssh --rsync-path="sudo rsync" pdns/opt/init_zone_once.sh {}:/opt/init_zone_once.sh
	@cat tmp/dns-servers | grep -v 'bench' | xargs -I{} ssh {} "sudo systemctl daemon-reload && sudo systemctl restart pdns"

.PHONY: replace-ISUCON13_POWERDNS_SUBDOMAIN_ADDRESS
replace-ISUCON13_POWERDNS_SUBDOMAIN_ADDRESS: tmp/dns-servers ## ISUCON13_POWERDNS_SUBDOMAIN_ADDRESSを192.168.0.12に置換
	@cat tmp/dns-servers | xargs -I{} ssh {} "sed -i '/ISUCON13_POWERDNS_SUBDOMAIN_ADDRESS/d' ~/env.sh && echo 'ISUCON13_POWERDNS_SUBDOMAIN_ADDRESS=\"192.168.0.12\"' >> ~/env.sh"

.PHONY: reset-pdns-zone
reset-pdns-zone: tmp/dns-servers ## PowerDNSのconfigを更新して再起動
	@cat tmp/dns-servers | xargs -I{} ssh {} "(pdnsutil delete-zone t.isucon.pw || echo 'ゾーンがなかった') && sudo rm -rf /opt/isunarabe-env-ipaddr.sh.lock"
	@cat tmp/dns-servers | xargs -I{} ssh {} "~/webapp/pdns/init_zone.sh"

################################################################################
# 最低限のセットアップ
################################################################################
.PHONY: setup-basic
setup-basic: ## 最低限のセットアップ
	@make setup-tools
	@make replace-nginx-conf
	@make clean-nginx-log-and-reload
	@make mysql-bind-address-0000
	@make create-mysql-user
	@make enable-mysql-slowquery-log

################################################################################
# プログラミング言語の切り替え
################################################################################
.PHONY: switch-golang
switch-golang: tmp/servers ## isupipeの言語をgolangにする(再起動)
	@cat tmp/servers | grep -v 'bench' | xargs -I{} ssh {} "systemctl list-units --type=service --all | grep isupipe | cut -d' ' -f3 | xargs -I{} sudo systemctl disable --now {}"
	@cat tmp/servers | grep -v 'bench' | xargs -I{} ssh {} "sudo systemctl enable --now isupipe-go"

.PHONY: switch-python
switch-python: tmp/servers ## isupipeの言語をpythonにする(再起動)
	@cat tmp/servers | grep -v 'bench' | xargs -I{} ssh {} "systemctl list-units --type=service --all | grep isupipe | cut -d' ' -f3 | xargs -I{} sudo systemctl disable --now {}"
	@cat tmp/servers | grep -v 'bench' | xargs -I{} ssh {} "sudo systemctl enable --now isupipe-python"

.PHONY: switch-ruby
switch-ruby: tmp/servers## isupipeの言語をrubyにする(再起動)
	@cat tmp/servers | grep -v 'bench' | xargs -I{} ssh {} "systemctl list-units --type=service --all | grep isupipe | cut -d' ' -f3 | xargs -I{} sudo systemctl disable --now {}"
	@cat tmp/servers | grep -v 'bench' | xargs -I{} ssh {} "sudo systemctl enable --now isupipe-ruby"

################################################################################
# Kaizen
################################################################################
.PHONY: kaizen
kaizen: ## 続きからやるためのやつ
	cat tmp/db-servers | xargs -I{} ssh {} "sudo mysql isupipe -e 'alter table livestream_tags add index livestream_id_idx (livestream_id);' || echo 'すでにある'"
	cat tmp/db-servers | xargs -I{} ssh {} "sudo mysql isupipe -e 'alter table icons add index user_id_idx (user_id);' || echo 'すでにある'"
	cat tmp/db-servers | xargs -I{} ssh {} "sudo mysql isupipe -e 'alter table themes add index user_id_idx (user_id);' || echo 'すでにある'"
	cat tmp/db-servers | xargs -I{} ssh {} "sudo mysql isupipe -e 'alter table livecomments add index livestream_id_idx (livestream_id);' || echo 'すでにある'"
	cat tmp/db-servers | xargs -I{} ssh {} "sudo mysql isupipe -e 'alter table livestreams add index user_id_idx (user_id);' || echo 'すでにある'"
	cat tmp/db-servers | xargs -I{} ssh {} "sudo mysql isupipe -e 'alter table reactions add index livestream_id_idx (livestream_id);' || echo 'すでにある'"
	cat tmp/db-servers | xargs -I{} ssh {} "sudo mysql isupipe -e 'alter table ng_words add index livestream_id_idx (livestream_id);' || echo 'すでにある'"
	cat tmp/db-servers | xargs -I{} ssh {} "sudo mysql isudns  -e 'alter table records add index name_idx (name);' || echo 'すでにある'"
	make replace-ISUCON13_POWERDNS_SUBDOMAIN_ADDRESS
	make replace-ISUCON13_MYSQL_DIALCONFIG_ADDRESS
	make rsync-pdns-and-restart
	make rsync-app-and-build-and-restart

################################################################################
# 分析
################################################################################
.PHONY: download-files-for-analysis
download-files-for-analysis: tmp/servers ## 分析用のファイルをダウンロード
	@bash scripts/download-files-for-analysis.sh

.PHONY: alp
alp: ## alpでnginxのログを分析(brew install alp)
	alp json --sort sum -r -o count,method,uri,min,avg,max,sum --file tmp/analysis/latest/nginx-access.log.* -m '/api/user/\w+/statistics,/api/user/\w+/icon,/api/user/\w+/theme,/api/livestream/\d+/livecomment,/api/livestream/\d+/reaction,/api/livestream/\d+/moderate,/api/livestream/\d+/report,/api/livestream/\d+/ngwords,/api/livestream/\d+/exit,/api/livestream/\d+/enter,/api/livestream/\d+/statistics,/api/livestream/\d+'

.PHONY: alp-each
alp-each: ## alpでnginxのログを分析(brew install alp)
	cat tmp/nginx-servers | xargs -I{} alp json --sort sum -r -o count,method,uri,min,avg,max,sum --file tmp/analysis/latest/nginx-access.log.{} -m '/api/user/\w+/statistics,/api/user/\w+/icon,/api/user/\w+/theme,/api/livestream/\d+/livecomment,/api/livestream/\d+/reaction,/api/livestream/\d+/moderate,/api/livestream/\d+/report,/api/livestream/\d+/ngwords,/api/livestream/\d+/exit,/api/livestream/\d+/enter,/api/livestream/\d+/statistics,/api/livestream/\d+'


.PHONY: pt-query-digest
pt-query-digest: ## pt-query-digestでスロークエリを分析(brew install percona-toolkit)
	pt-query-digest --limit 10 tmp/analysis/latest/mysql-slow.log.*

.PHONY: pt-query-digest-each
pt-query-digest-each: ## pt-query-digestでスロークエリを分析(brew install percona-toolkit)
	cat tmp/db-servers | xargs -I{} bash -c 'pt-query-digest --limit 10 tmp/analysis/latest/mysql-slow.log.{} > tmp/pt-query-digest.{}'

.PHONY: clean-log
clean-log: ## MySQL, Nginxのログをリセットする
	@bash scripts/clean-log.sh
	@cat tmp/webapp-servers | xargs -I{} ssh {} "rm -rf /home/isucon/webapp/public/images/*"

################################################################################
# NewRelic
################################################################################
.PHONY: add-newrelic-user-for-mysql
add-newrelic-user-for-mysql: tmp/servers ## MySQLにnewrelicユーザーを追加
	@cat tmp/servers | grep -v 'bench' | xargs -I{} ssh {} "sudo mysql -e \"create user if not exists 'newrelic'@'localhost' identified by 'newrelic';\""
	@cat tmp/servers | grep -v 'bench' | xargs -I{} ssh {} "sudo mysql -e \"grant replication client on *.* to 'newrelic'@'localhost';\""
	@cat tmp/servers | grep -v 'bench' | xargs -I{} ssh {} "sudo mysql -e \"grant select on *.* to 'newrelic'@'localhost';\""

.PHONY: install-newrelic
install-newrelic: tmp/servers ## newrelicを導入
	@cat tmp/servers | xargs -I{} ssh {} "(command -v /usr/local/bin/newrelic && /usr/local/bin/newrelic --version) || (curl -Ls https://download.newrelic.com/install/newrelic-cli/scripts/install.sh | bash)"
	@cat tmp/servers | xargs -I{} ssh {} "sudo NEW_RELIC_API_KEY=${NEW_RELIC_API_KEY} NEW_RELIC_LICENSE_KEY=${NEW_RELIC_LICENSE_KEY} NEW_RELIC_ACCOUNT_ID=${NEW_RELIC_ACCOUNT_ID} /usr/local/bin/newrelic install -y"

.PHONY: install-newrelic-apm-for-ptyhon
install-newrelic-apm-for-ptyhon: tmp/servers ## pythonアプリにnewrelic APMを導入
	@envsubst '$$NEW_RELIC_LICENSE_KEY' < newrelic/python/newrelic.template.ini > tmp/newrelic.ini
	@cat tmp/servers | grep -v 'bench' | xargs -I{} scp tmp/newrelic.ini {}:/home/isucon/webapp/python/newrelic.ini
	@cat tmp/servers | grep -v 'bench' | xargs -I{} ssh {} "sed -i '/NEW_RELIC_CONFIG_FILE/d' /home/isucon/env.sh && echo 'NEW_RELIC_CONFIG_FILE=\"/home/isucon/webapp/python/newrelic.ini\"' >> /home/isucon/env.sh"
	@cat tmp/servers | grep -v 'bench' | xargs -I{} ssh {} "cd /home/isucon/webapp/python && /home/isucon/local/python/bin/pipenv install newrelic"
	@cat tmp/servers | grep -v 'bench' | head -n1 | xargs -I{} scp {}:/home/isucon/webapp/python/app.py tmp/app.py
	@echo 'scp remote:/home/isucon/webapp/python/app.py local:./tmp/app.py'
	@sed '/newrelic/d' tmp/app.py > tmp/temp-app.py
	@awk '/from flask import Flask/ { \
	  print "import newrelic.agent"; \
	  print "newrelic.agent.initialize('\''/home/isucon/webapp/python/newrelic.ini'\'')"; \
	} \
	{print}' tmp/temp-app.py > tmp/app.py
	@cat tmp/servers | grep -v 'bench' | xargs -I{} scp tmp/app.py {}:/home/isucon/webapp/python/app.py
	@rm tmp/app.py tmp/temp-app.py tmp/newrelic.ini

.PHONY: install-newrelic-apm-for-ruby
install-newrelic-apm-for-ruby: tmp/servers ## rubyアプリにnewrelic APMを導入
	@cat tmp/servers | grep -v 'bench' | xargs -I{} ssh isucon@{} -i ${SSH_KEY_PATH} "grep 'newrelic' /home/isucon/webapp/ruby/Gemfile || (cd /home/isucon/webapp/ruby && /home/isucon/local/ruby/bin/bundle add newrelic_rpm newrelic-infinite_tracing)"
	@envsubst '$$NEW_RELIC_LICENSE_KEY' < newrelic/ruby/newrelic.template.yml > tmp/newrelic.yml
	@cat tmp/servers | grep -v 'bench' | xargs -I{} scp -i ${SSH_KEY_PATH} tmp/newrelic.yml isucon@{}:/home/isucon/webapp/ruby/newrelic.yml
	@cat tmp/servers | grep -v 'bench' | xargs -I{} ssh isucon@{} -i ${SSH_KEY_PATH} "sed -i '/NEW_RELIC_CONFIG_FILE/d' /home/isucon/env.sh"
	@cat tmp/servers | grep -v 'bench' | head -n1 | xargs -I{} scp -i ${SSH_KEY_PATH} isucon@{}:/home/isucon/webapp/ruby/app.rb tmp/app.rb
	@echo 'scp remote:/home/isucon/webapp/ruby/app.rb local:./tmp/app.rb'
	@sed '/newrelic/d' tmp/app.rb > tmp/temp-app.rb
	@awk '/require '"'"'sinatra\/json'"'"'/ {print; print "require \"newrelic_rpm\""; print "require \"newrelic/infinite_tracing\""; next} 1' tmp/temp-app.rb > tmp/app.rb
	@cat tmp/servers | grep -v 'bench' | xargs -I{} scp -i ${SSH_KEY_PATH} tmp/app.rb isucon@{}:/home/isucon/webapp/ruby/app.rb
	@rm tmp/temp-app.rb tmp/app.rb tmp/newrelic.yml

################################################################################
# エラー文言
################################################################################
cloudformation.yml:
	@echo 'ISUNARABEからcloudformation.ymlをDLしてください' >&2
	exit 1

tmp/servers:
	@echo 'isu-1' > tmp/servers
	@echo 'isu-2' >> tmp/servers
	@echo 'isu-3' >> tmp/servers
	@echo 'isu-bench' >> tmp/servers

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
