# Copyright (c) 2015-2023, NVIDIA CORPORATION.
# SPDX-License-Identifier: Apache-2.0

FROM ubuntu:22.04 as base

RUN    apt-get update \
    && apt-get dist-upgrade -y

ARG TimeZone=America/Los_Angeles
RUN ln -snf /usr/share/zoneinfo/${TimeZone} /etc/localtime
RUN echo ${TimeZone} > /etc/timezone

RUN DEBIAN_FRONTEND="noninteractive" apt-get install -y tzdata

FROM base as buildable

RUN    apt-get update \
    && apt-get install -y \
                        curl \
                        dnsutils \
                        fuse \
                        git \
                        iputils-ping \
                        jq \
                        make \
                        s3cmd \
                        tree \
                        vim \
                        wget

ARG GolangVersion=1.21.6
ENV GolangBasename="go${GolangVersion}.linux-amd64.tar.gz"
ENV GolangURL="https://golang.org/dl/${GolangBasename}"
WORKDIR /tmp
RUN wget -nv ${GolangURL}
RUN tar -C /usr/local -xzf $GolangBasename
ENV PATH $PATH:/usr/local/go/bin
RUN git clone https://github.com/go-delve/delve
WORKDIR /tmp/delve
RUN go build github.com/go-delve/delve/cmd/dlv
RUN cp dlv /usr/local/go/bin/.

RUN echo '#!/bin/bash'                  >  /root/.bashrc_additions
RUN echo 'export PS1="\w$ "'            >> /root/.bashrc_additions
RUN echo 'export GOPATH=${HOME}/go'     >> /root/.bashrc_additions
RUN echo 'export GOBIN=${GOPATH}/bin'   >> /root/.bashrc_additions
RUN echo 'export PATH=${GOBIN}:${PATH}' >> /root/.bashrc_additions

RUN echo ""                      >> /root/.bashrc
RUN echo ". ~/.bashrc_additions" >> /root/.bashrc

RUN echo "[default]"                 >  /root/.s3cfg
RUN echo "access_key  = test:tester" >> /root/.s3cfg
RUN echo "host_base   = swift:8080"  >> /root/.s3cfg
RUN echo "host_bucket = AUTH_test"   >> /root/.s3cfg
RUN echo "secret_key  = testing"     >> /root/.s3cfg
RUN echo "use_https   = False"       >> /root/.s3cfg

WORKDIR /fission

COPY . .

RUN git config --global --add safe.directory /fission

FROM buildable as dev

WORKDIR /
RUN rm -rf /fission

VOLUME /fission
WORKDIR /fission
