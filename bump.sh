#!/bin/bash

OLD=$(cat ./version | cut -d'.' -f1)
NEW=$(expr $OLD + 1)
echo $NEW > version
git add version
git commit -am "increased version number"
git push origin master
git tag $NEW -m "$NEW"
git push --tags
