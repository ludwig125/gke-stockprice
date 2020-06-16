# ローカルでの開発環境構築

## gcloudインストール

[Linux 用のクイックスタート  \|  Cloud SDK のドキュメント  |  Google Cloud](https://cloud.google.com/sdk/docs/quickstart-linux?hl=ja)

## mysqlのインストール

```bash
$ sudo apt install mysql-server mysql-client

$mysql --version
mysql  Ver 14.14 Distrib 5.7.30, for Linux (x86_64) using  EditLine wrapper
```

起動に使用するmysqlユーザーのホームディレクトリが存在しないとmysql serverを立ち上げられないので/etc/passwdに以下を追加

```bash
$ sudo usermod -d /var/lib/mysql mysql
```

/etc/passwdに以下が追加されている

```bash
mysql:x:111:115:MySQL Server,,,:/var/lib/mysql:/bin/false
```

mysql server の起動

```bash
$ sudo service mysql start
```

ubuntu18.04でデフォルトのmysql5.7ではroot権限でないと接続できないらしい
これだと個人開発環境では不便なので、sudoいらなくさせる

```bash
mysql > ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '';
mysql > FLUSH PRIVILEGES;
```

これでsudoもpasswordも不要になる

## go test

mysqlが起動してればこれができる
```
$go test -v ./... -p 1 -count=1
```
go testはデフォルトではパラレルでテストを実行してしまうので、
Mysqlのデータが競合しないように`-p 1`として並列数を１にしている

## circleci cli

cliのインストール(以下はWSL2 Ubuntu18.04でやった場合)

- 参考：https://circleci.com/docs/2.0/local-cli/

```
$curl -fLSs https://raw.githubusercontent.com/CircleCI-Public/circleci-cli/master/install.sh | s
udo bash

$which circleci
/usr/local/bin/circleci
```

バリデーションチェック
```
$circleci config validate -c .circleci/config.yml
```

# 本番CloudSQL

## instanceの作成

```bash
$ gcloud sql instances create gke-stockprice-cloudsql-prod --tier=db-f1-micro --region=us-central1 --storage-auto-increase --no-backup
```

instanceの確認
```
gcloud sql instances list
```

passwordの設定

```bash
$ gcloud sql users set-password root --host=% --instance=gke-stockprice-cloudsql-prod --prompt-for-password
Instance Password: # ここに設定したいパスワードを入力
Updating Cloud SQL user...done.
```

connectionNameの確認方法

```bash
$ gcloud sql instances describe gke-stockprice-cloudsql-prod | grep connectionName
connectionName: gke-stockprice:us-central1:gke-stockprice-cloudsql-prod
```

## Databaseの準備

cloud_sql_proxy経由で接続する

cloud_sql_proxy取得

- <https://cloud.google.com/sql/docs/mysql/connect-admin-proxy?hl=ja>

cloud_sql_proxyで上のconnectionNameとportを指定

- ローカルのMySQLのPort(3306)ととかぶらないように3307を使用する

```bash
$ ./cloud_sql_proxy -instances=gke-stockprice:us-central1:gke-stockprice-cloudsql-prod=tcp:3307
```

別に端末を開いてMySQLコマンドで接続

```bash
$ mysql -u root --host 127.0.0.1 --port 3307 -p
Enter password:   // <- passwordを入力
```

database作成
```
CREATE DATABASE IF NOT EXISTS stockprice;
```

database確認
```
mysql> show databases;
+--------------------+
| Database           |
+--------------------+
| information_schema |
| mysql              |
| performance_schema |
| stockprice         |
| sys                |
+--------------------+
5 rows in set (0.13 sec)

mysql>
```

table作成

daily
```bash
CREATE TABLE IF NOT EXISTS stockprice.daily (
		code VARCHAR(10) NOT NULL,
		date VARCHAR(10) NOT NULL,
		open VARCHAR(15),
		high VARCHAR(15),
		low VARCHAR(15),
		close VARCHAR(15),
		turnover VARCHAR(15),
		modified VARCHAR(15),
		PRIMARY KEY( code, date )
	);
```

movingavg
```bash
CREATE TABLE IF NOT EXISTS stockprice.movingavg (
        code VARCHAR(10) NOT NULL,
        date VARCHAR(10) NOT NULL,
        moving3 DOUBLE,
        moving5 DOUBLE,
        moving7 DOUBLE,
        moving10 DOUBLE,
        moving20 DOUBLE,
        moving60 DOUBLE,
        moving100 DOUBLE,
        PRIMARY KEY( code, date )
	);
```

table確認
```
mysql> use stockprice
Reading table information for completion of table and column names
You can turn off this feature to get a quicker startup with -A

Database changed
mysql> show tables;
+----------------------+
| Tables_in_stockprice |
+----------------------+
| daily                |
| movingavg            |
+----------------------+
2 rows in set (0.13 sec)

mysql>
```

# circleciのジョブをAPIから実行

```bash
$ curl -XPOST https://circleci.com/api/v1.1/project/github/ludwig125/gke-stockprice/tree/master --user "${CIRCLE_API_USER_TOKEN}:" --header "Content-Type: application/json" -d '{
  "build_parameters": {
    "CIRCLE_JOB": "create_gke_cluster"
  }
}'
```

# GKE操作

### 作成

クラスタの作成
```
CLUSTER_NAME=gke-stockprice-cluster-prod
COMPUTE_ZONE=us-central1-f
MACHINE_TYPE=g1-small
NUM_NODES=2

gcloud --quiet container clusters create $CLUSTER_NAME \
                --machine-type=$MACHINE_TYPE --disk-size 10 --zone $COMPUTE_ZONE \
                --num-nodes=$NUM_NODES
```
- `-quiet`をつけることで作成時の「yes/no」の入力を省略できる

kubectlが使えるようになるための認証

```
gcloud auth login
PROJECT_NAME=gke-stockprice
CLUSTER_NAME=gke-stockprice-cluster-prod
COMPUTE_ZONE=us-central1-f
gcloud config set project $PROJECT_NAME
gcloud config set container/cluster $CLUSTER_NAME
gcloud config set compute/zone ${COMPUTE_ZONE}
gcloud container clusters get-credentials $CLUSTER_NAME
kubectl get pods
```

#### 実行中のコンテナへのシェルを取得

```bash
$ kubectl exec -it `kubectl get pods | grep stockprice | awk '{print $1}'` --container=gke-stockprice-container /bin/ash
/go #
```
