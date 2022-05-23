#!/bin/bash

helm package .
helm repo index . --url https://fi.6shore.net/helm/ --merge index.yaml


mc cp index.yaml obj/static/fi.6shore.net/helm/
mc cp *.tgz obj/static/fi.6shore.net/helm/
rm -v *.tgz
