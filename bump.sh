#!/bin/bash

CHART="./helm/naisd/Chart.yaml"

OLD=$(cat ./version | cut -d'.' -f1)
NEW=$(expr $OLD + 1).0.0

echo "$NEW" > version
grep -v "version: " $CHART > temp && mv temp $CHART && rm -f temp && echo "version: $NEW" >> $CHART

git add version $CHART
git commit -am "increased version number to $NEW [skip ci]"
git push origin master
git tag $NEW -m "$NEW"
git push --tags
