#!/bin/bash -e
if [[ $# -ne 1 ]]; then
    echo "Usage: $0 <version>" 1>&2;
    exit 1;
fi;

VERSION="$1"

basedir=$(cd $(dirname $0)/..; /bin/pwd)
if ! docker build --tag kinesis-log-streamer:centos7 --file $basedir/packaging/centos7.dockerfile --build-arg VERSION="$VERSION" $basedir; then
    echo "Docker build failed" 1>&2;
    exit 1;
fi;

if ! docker run --mount type=bind,src=$basedir/export,dst=/export kinesis-log-streamer:centos7 find /root/rpmbuild/SRPMS /root/rpmbuild/RPMS -name \*.rpm -exec cp '{}' /export ';'; then
    echo "Failed to copy RPM files from Docker container" 1>&2;
    exit 1;
fi;
