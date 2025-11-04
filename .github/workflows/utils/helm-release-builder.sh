#!/bin/bash
# -------------------------------------------------------------------------------------
#
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
#
# --------------------------------------------------------------------------------------

# Fail the script when a subsuquent command or pipe redirection fails
set -e
set -o pipefail

if [ "$#" -ne 3 ] || ! [ -d "$3" ]; then
  echo "Error: Invalid or insufficient arguements!" >&2
  echo "Usage: bash $0 <GITHUB_USERNAME> <GITHUB_USER_EMAIL> <GITHUB_TOKEN> <PRODUCT_REPO_DIR>" >&2
  exit 1
fi

# Check if jq and gh exists in the system
command -v jq >/dev/null 2>&1 || { echo >&2 "Error: $0 script requires 'jq' for JSON Processing.  Aborting as not found."; exit 1; }
command -v gh >/dev/null 2>&1 || { echo >&2 "Error: $0 script requires 'gh' to call GitHub APIs.  Aborting as not found."; exit 1; }

# Variables
GIT_USERNAME=$1
GIT_EMAIL=$2
GIT_TOKEN=$3
WORK_DIR=$4
CHART_YAML="${WORK_DIR}/install/helm/Chart.yaml"

# Login to github cli with token.
echo "${GIT_TOKEN}" | gh auth login --with-token

# Read the tag version from the Chart.yaml
TAG_VERSION_TMP=$(grep 'version:' "${CHART_YAML}")
TAG_VERSION=${TAG_VERSION_TMP//*version: /}

# Exporting variable current helm pack version
echo "::set-output name=CURRENT_TAG_VERSION::${TAG_VERSION}"

echo "Tag version: ${TAG_VERSION}"

## Increment tag version to next tag version.
MAJOR=$(echo "${TAG_VERSION}" | cut -d. -f1)
MINOR=$(echo "${TAG_VERSION}" | cut -d. -f2)
PATCH=$(echo "${TAG_VERSION}" | cut -d. -f3)
PATCH=$((PATCH + 1))
NEW_TAG_VERSION=$MAJOR.$MINOR.$PATCH

echo "New release tag version: ${NEW_TAG_VERSION}"

# Set new release tag.
TAG="v${TAG_VERSION}-helm"
TAG_NAME="helm-identity-customer-data-service-${TAG}"
echo "Release tag: ${TAG}"

# Release the tag.
gh release create --target main --title "${TAG_NAME}" -n "" "${TAG}";

git -C "${WORK_DIR}" config user.email "${GIT_EMAIL}"
git -C "${WORK_DIR}" config user.name "${GIT_USERNAME}"
git -C "${WORK_DIR}" pull

# Update the version in Chart.yaml
sed -i "s/version: ${TAG_VERSION}/version: ${NEW_TAG_VERSION}/" "${CHART_YAML}"
# Push new release version to Chart.yaml
git -C "${WORK_DIR}" add "${CHART_YAML}"
git -C "${WORK_DIR}" commit -m "Update Chart version to - v${NEW_TAG_VERSION}"
git -C "${WORK_DIR}" push
