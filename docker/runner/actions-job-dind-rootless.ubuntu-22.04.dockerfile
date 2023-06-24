FROM ubuntu:22.04

ARG RUNNER_VERSION=2.305.0

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -y \
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
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean

# install slirp4netns v1.2.0
RUN curl -o /usr/bin/slirp4netns --fail -L https://github.com/rootless-containers/slirp4netns/releases/download/v1.2.0/slirp4netns-$(uname -m) \
    && chmod +x /usr/bin/slirp4netns

# ref https://github.com/actions/actions-runner-controller/issues/2143#issuecomment-1424462740
RUN update-alternatives --set iptables /usr/sbin/iptables-legacy
RUN update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy

# Use 1001 for compatibility with GitHub-hosted runners
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
RUN chmod +x /usr/local/bin/docker

COPY entrypoint-dind-rootless.sh startup.sh /usr/bin/
RUN chmod +x /usr/bin/entrypoint-dind-rootless.sh /usr/bin/startup.sh

ENV PATH="/home/runner/bin:${HOME}/.local/bin:${PATH}"
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
