CONFIG_PATH=${HOME}/.proglog

.PHONY: init
init:
	mkdir -p ${CONFIG_PATH}

.PHONY: gencert
gencert:
	# ca.pemとca-key.pemの生成
	cfssl gencert \
		-initca test/ca-csr.json | cfssljson -bare ca
	# 違うCAで証明書を発行してみる
	# cfssl gencert \
	# 	-initca test/ca-csr2.json | cfssljson -bare ca2

	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=test/ca-config.json \
		-profile=server \
		test/server-csr.json | cfssljson -bare server

	# 違うCAで証明書を発行してみる
	# cfssl gencert \
	# 	-ca=ca2.pem \
	# 	-ca-key=ca2-key.pem \
	# 	-config=test/ca-config.json \
	# 	-profile=client \
	# 	test/client-csr.json | cfssljson -bare client

	# 認可のテストのために複数のクライアントを用意する
	# 同じ設定ファイルを使用するがcn(common name)を指定して別のクライアントを生成
	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=test/ca-config.json \
		-profile=client \
		-cn="root" \
		test/client-csr.json | cfssljson -bare root-client

	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=test/ca-config.json \
		-profile=client \
		-cn="nobody" \
		test/client-csr.json | cfssljson -bare nobody-client

	mv *.pem *.csr ${CONFIG_PATH}

$(CONFIG_PATH)/model.conf:
	cp test/model.conf $(CONFIG_PATH)/model.conf

$(CONFIG_PATH)/policy.csv:
	cp test/policy.csv $(CONFIG_PATH)/policy.csv

# -raceはデータ競合が発生していないかを検出するためのフラグ
# データ競合:複数のゴールーチンが同じメモリ領域に同時にアクセスすること
.PHONY: test
test: $(CONFIG_PATH)/policy.csv $(CONFIG_PATH)/model.conf
	go test -race ./...

.PHONY: compile
compile:
	protoc api/v1/*.proto \
		--go_out=. \
		--go-grpc_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		--proto_path=.
