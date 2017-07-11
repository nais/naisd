#!/bin/bash

# bump version number by one
# uses first arg as token for github push, else use local default

TOKEN=${1}
CHART="./helm/naisd/Chart.yaml"

OLD=$(cat ./version | cut -d'.' -f1)
NEW=$(expr $OLD + 1).0.0

echo "$NEW" > version
grep -v "version: " $CHART > temp && mv temp $CHART && rm -f temp && echo "version: $NEW" >> $CHART

git add version $CHART
git commit -am "increased version number to $NEW [skip ci]"

if [[ -n ${TOKEN} ]]; then
    git push https://${TOKEN}@github.com/nais/naisd HEAD:master
else
    git push origin master
fi

echo $NEW
