FROM centos:7
ARG VERSION=0.2.0-0
RUN yum update -y && yum install -y httpd
COPY export/kinesis-log-streamer-$VERSION.el7.x86_64.rpm /root/
RUN yum install -y /root/kinesis-log-streamer-$VERSION.el7.x86_64.rpm
COPY functest/apachelogs/httpd.conf /etc/httpd/conf/
