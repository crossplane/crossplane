#!/usr/bin/env bash

set -e
trap 'exit 1' SIGINT

echo -n "waiting for mysql endpoint..." >&2
while kubectl -n get mysqlservers -o yaml | grep -q 'items: \[\]'; do
  echo -n "." >&2
  sleep 5
done
echo "done" >&2

export MYSQL_NAME=$(kubectl -n get mysqlservers -o=jsonpath='{.items[0].metadata.name}')

sed "s/MYSQL_NAME/$MYSQL_NAME/g" vnet-rule.yaml | kubectl apply -f -
