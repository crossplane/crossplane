#!/bin/bash -e

# Copyright 2016 The Upbound Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# get the build environment variables from the special build.vars target in the main makefile
eval $(make --no-print-directory -C ${scriptdir}/.. build.vars)

KUBEADM_DIND_DIR=${CACHE_DIR}/kubeadm-dind

CROSS_IMAGE=${BUILD_REGISTRY}/cross-amd64
CROSS_IMAGE_VOLUME=cross-volume
CROSS_RSYNC_PORT=10873

function start_rsync_container() {
    docker run \
        -d \
        -e OWNER=root \
        -e GROUP=root \
        -e MKDIRS="/volume/go/src/${PROJECT_REPO}" \
        -p ${CROSS_RSYNC_PORT}:873 \
        -v ${CROSS_IMAGE_VOLUME}:/volume \
        --entrypoint "/tini" \
        ${CROSS_IMAGE} \
        -- /build/rsyncd.sh
}

function wait_for_rsync() {
    # wait for rsync to come up
    local tries=100
    while (( ${tries} > 0 )) ; do
        if rsync "rsync://localhost:${CROSS_RSYNC_PORT}/"  &> /dev/null ; then
            return 0
        fi
        tries=$(( ${tries} - 1 ))
        sleep 0.1
    done
    echo ERROR: rsyncd did not come up >&2
    exit 1
}

function stop_rsync_container() {
    local id=$1

    docker stop ${id} &> /dev/null || true
    docker rm ${id} &> /dev/null || true
}

function run_rsync() {
    local src=$1
    shift

    local dst=$1
    shift

    # run the container as an rsyncd daemon so that we can copy the
    # source tree to the container volume.
    local id=$(start_rsync_container)

    # wait for rsync to come up
    wait_for_rsync || stop_rsync_container ${id}

    # NOTE: add --progress to show files being syncd
    rsync \
        --archive \
        --delete \
        --prune-empty-dirs \
        "$@" \
        $src $dst || { stop_rsync_container ${id}; return 1; }

    stop_rsync_container ${id}
}

function rsync_host_to_container() {
    run_rsync ${scriptdir}/.. rsync://localhost:${CROSS_RSYNC_PORT}/volume/go/src/${PROJECT_REPO} "$@"
}

function rsync_container_to_host() {
    run_rsync rsync://localhost:${CROSS_RSYNC_PORT}/volume/go/src/${PROJECT_REPO}/ ${scriptdir}/.. "$@"
}
