#!/usr/bin/env bash

set -o errexit

repo=${REPOSITORY:-nuclio/nuclio}
token=${TOKEN}
commit_sha=${SHA:$(git rev-parse --verify HEAD)}

workflow_name="nuclio-docker-images"
workflow_file=ci.yaml
workflow_runs_url="https://api.github.com/repos/$repo/actions/workflows/$workflow_file/runs"
output_filename=nuclio_docker_images.zip

echo "
    rep:                        $repo
    commit_sha:                 $commit_sha
"

run_id=$(curl --retry 3 -s -H 'Accept: application/vnd.github.antiope-preview+json' ${workflow_runs_url} \
    | jq ".workflow_runs[]  | select(.head_sha == \"$commit_sha\") | .id")
echo "Run ID: $run_id"

artifacts_url="https://api.github.com/repos/$repo/actions/runs/$run_id/artifacts"
artifact_download_url=$(curl --retry 3 -s -H 'Accept: application/vnd.github.antiope-preview+json' ${artifacts_url} \
    | jq -r ".artifacts[] | select(.name == \"$workflow_name\") | .archive_download_url")
echo "Artifact download URL: $artifact_download_url"

echo "Downloading from: $artifact_download_url"
curl --retry 3 -H "Authorization: token $token" -L -o ${output_filename} ${artifact_download_url}

unzip ${output_filename}
rm ${output_filename}
