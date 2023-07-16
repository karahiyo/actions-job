FROM ubuntu:20.04

ARG RUNNER_VERSION=2.305.0
ARG DOCKER_VERSION=20.10.23

ARG RUNNER_UID=1000
ARG DOCKER_GID=1001

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -y \
    && apt-get install -y software-properties-common \
    && add-apt-repository -y ppa:git-core/ppa \
    && apt-get update -y \
    && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    curl \
    dnsutils \
    ftp \
    dumb-init \
    git \
    iproute2 \
    iputils-ping \
    iptables \
    jq \
    libunwind8 \
    kmod \
    locales \
    netcat \
    net-tools \
    openssh-client \
    parallel \
    python3-pip \
    rsync \
    shellcheck \
    software-properties-common \
    sudo \
    telnet \
    time \
    tzdata \
    uidmap \
    upx \
    wget \
    zstd \
    && ln -sf /usr/bin/python3 /usr/bin/python \
    && ln -sf /usr/bin/pip3 /usr/bin/pip \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean

# Runner user
RUN adduser --disabled-password --gecos "" --uid $RUNNER_UID runner \
    && groupadd docker --gid $DOCKER_GID \
    && usermod -aG sudo runner \
    && usermod -aG docker runner \
    && echo "%sudo   ALL=(ALL:ALL) NOPASSWD:ALL" > /etc/sudoers \
    && echo "Defaults env_keep += \"DEBIAN_FRONTEND\"" >> /etc/sudoers

ENV HOME=/home/runner

ENV RUNNER_ASSETS_DIR=/runnertmp
RUN mkdir -p "${RUNNER_ASSETS_DIR}" \
    && cd "${RUNNER_ASSETS_DIR}" \
    && curl -sfLo runner.tar.gz -L https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz \
    && tar xzf ./runner.tar.gz \
    && rm ./runner.tar.gz \
    && ./bin/installdependencies.sh

ENV RUNNER_TOOL_CACHE=/opt/hostedtoolcache
RUN mkdir /opt/hostedtoolcache \
    && chgrp runner /opt/hostedtoolcache \
    && chmod g+rwx /opt/hostedtoolcache

RUN set -vx; \
    export ARCH=x86_64 \
    && curl -fLo docker.tgz https://download.docker.com/linux/static/stable/${ARCH}/docker-${DOCKER_VERSION}.tgz \
    && tar zxvf docker.tgz \
    && install -o root -g root -m 755 docker/* /usr/bin/ \
    && rm -rf docker docker.tgz

# Copy the docker shim which propagates the docker MTU to underlying networks
# to replace the docker binary in the PATH.
COPY docker-shim.sh /usr/local/bin/docker
RUN chmod +x /usr/local/bin/docker

COPY entrypoint-dind.sh startup.sh /usr/bin/
RUN chmod +x /usr/bin/entrypoint-dind.sh /usr/bin/startup.sh

VOLUME /var/lib/docker

ENV PATH="${PATH}:${HOME}/.local/bin"
ENV ImageOS=ubuntu20

RUN echo "PATH=${PATH}" > /etc/environment \
    && echo "ImageOS=${ImageOS}" >> /etc/environment

USER $RUNNER_UID

ENTRYPOINT ["/bin/bash", "-c"]
CMD ["entrypoint-dind.sh"]
