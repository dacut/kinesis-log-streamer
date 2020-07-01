FROM centos:7
RUN yum update -y && yum install -y epel-release && yum update -y && yum install -y golang
RUN yum install -y rpm-build
ARG VERSION
RUN mkdir -p /root/kinesis-log-streamer-$VERSION
COPY LICENSE README.md go.mod go.sum *.go /root/kinesis-log-streamer-$VERSION/
RUN mkdir -p /root/rpmbuild/SOURCES
RUN tar -C /root -c -z -f /root/rpmbuild/SOURCES/kinesis-log-streamer-src-$VERSION.tar.gz kinesis-log-streamer-$VERSION
RUN mkdir -p /root/rpmbuild/SPECS
COPY ./packaging/centos7.spec /root/rpmbuild/SPECS/kinesis-log-streamer.spec
WORKDIR /root/rpmbuild/SPECS
RUN rpmbuild -D '%_topdir /root/rpmbuild' -D "%version $VERSION" -ba ./kinesis-log-streamer.spec
VOLUME /export
