#!/bin/bash
# Exit on any error
set -e

cd `dirname $0`

# CLOUDSQL_INSTANCE="gke-stockprice-cloudsql-prod"
# CLUSTER_NAME="gke-stockprice-cluster-prod"
# COMPUTE_ZONE="us-central1-a"
# MACHINE_TYPE="g1-small"
# NUM_NODES=2

# # cloud sql instanceの存在確認
# # gcloud sql instances listの結果が0だと、`Listed 0 items.`が出力されるので/dev/nullに捨てる
# if [ `gcloud sql instances list 2> /dev/null | grep $CLOUDSQL_INSTANCE | wc -l` -eq 0 ]; then
#     echo "failed to find cloudsql instance:" $CLOUDSQL_INSTANCE
#     return 1
# fi

# if [ `gcloud container clusters list | grep $CLUSTER_NAME | wc -l` == 1 ]; then
#     # clusterが存在する場合、ERRORになっていないか確認
#     if [ `gcloud container clusters describe $CLUSTER_NAME | grep 'ERROR' | wc -l` != 0 ]; then
#         # うまく作れていないときは消してからもう一度作る
#         gcloud --quiet container clusters delete $CLUSTER_NAME
#         gcloud --quiet container clusters create $CLUSTER_NAME \
#         --machine-type=$MACHINE_TYPE --disk-size 10 --zone $COMPUTE_ZONE \
#         --num-nodes=$NUM_NODES
#     fi
# elif [ `gcloud container clusters list | grep $CLUSTER_NAME | wc -l` == 0 ]; then
#     # clusterが存在しない場合はcreate
#     # --quiet をつけないと作成するかどうかy/nの入力を求める表示が出る
#     gcloud --quiet container clusters create $CLUSTER_NAME \
#     --machine-type=$MACHINE_TYPE --disk-size 10 --zone $COMPUTE_ZONE \
#     --num-nodes=$NUM_NODES
# else
#     echo "$CLUSTER_NAME cluster already exists"
# fi

