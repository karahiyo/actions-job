FROM ubuntu:22.04

ARG RUNNER_VERSION=2.304.0

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get -y update \
    && apt-get install -y software-properties-common \
    && add-apt-repository -y ppa:git-core/ppa \
    && apt-get update -y \
    && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    dbus-user-session \
    git \
    iproute2 \
    iptables \
    jq \
    kmod \
    locales \
    sudo \
    uidmap \
    slirp4netns \
    && rm -rf /var/lib/apt/lists/*

# ref https://github.com/actions/actions-runner-controller/issues/2143#issuecomment-1424462740
RUN update-alternatives --set iptables /usr/sbin/iptables-legacy

ARG RUNNER_USER_UID=1001
RUN adduser --disabled-password --gecos "" --uid $RUNNER_USER_UID runner

ENV HOME=/home/runner

## Set-up subuid and subgid so that "--userns-remap=default" works
RUN set -eux; \
    addgroup --system dockremap; \
    adduser --system --ingroup dockremap dockremap; \
    echo 'dockremap:165536:65536' >> /etc/subuid; \
    echo 'dockremap:165536:65536' >> /etc/subgid

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

# Make the rootless runner directory executable
RUN mkdir /run/user/1000 \
    && chown runner:runner /run/user/1000 \
    && chmod a+x /run/user/1000

RUN mkdir -p /home/runner/.local/share \
    && chmod 755 /home/runner/.local/share \
    && chown runner:runner /home/runner/.local/share

# Copy the docker shim which propagates the docker MTU to underlying networks
# to replace the docker binary in the PATH.
COPY docker-shim.sh /usr/local/bin/docker

COPY entrypoint-dind-rootless.sh startup.sh /usr/bin/
RUN chmod +x /usr/bin/entrypoint-dind-rootless.sh /usr/bin/startup.sh

ENV PATH="/home/runner/bin:${PATH}"
ENV ImageOS=ubuntu22
ENV XDG_RUNTIME_DIR=/run/user/1000
ENV DOCKER_HOST=unix:///run/user/1000/docker.sock

RUN echo "PATH=${PATH}" > /etc/environment \
    && echo "ImageOS=${ImageOS}" >> /etc/environment \
    && echo "DOCKER_HOST=${DOCKER_HOST}" >> /etc/environment \
    && echo "XDG_RUNTIME_DIR=${XDG_RUNTIME_DIR}" >> /etc/environment

USER runner

RUN export SKIP_IPTABLES=1 \
    && curl -fsSL https://get.docker.com/rootless | sh \
    && /home/runner/bin/docker -v

ENTRYPOINT ["/bin/bash", "-c"]
CMD ["entrypoint-dind-rootless.sh"]
