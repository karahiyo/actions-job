FROM ubuntu:22.04

ARG RUNNER_VERSION=2.304.0

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get -y update \
    && apt-get install -y \
    ca-certificates \
    curl \
    locales \
    git \
    iproute2 \
    iptables \
    jq \
    locales \
    sudo \
    kmod \
    dbus-user-session \
    uidmap

ARG RUNNER_USER_UID=1001
RUN adduser --disabled-password --gecos "" --uid $RUNNER_USER_UID runner

ENV HOME=/home/runner

ENV RUNNER_ASSETS_DIR=/runnertmp
RUN mkdir -p "${RUNNER_ASSETS_DIR}" \
    && cd "${RUNNER_ASSETS_DIR}" \
    && curl -sfLo runner.tar.gz -L https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz \
    && tar xzf ./runner.tar.gz \
    && rm ./runner.tar.gz \
    && ./bin/installdependencies.sh \
    && mv ./externals ./externalstmp

COPY entrypoint-dood-rootless.sh startup.sh wait-for-it.sh /usr/bin/
RUN chmod +x /usr/bin/entrypoint-dood-rootless.sh /usr/bin/startup.sh /usr/bin/wait-for-it.sh

ENV PATH="${PATH}:${HOME}/.local/bin:${HOME}/bin"

ENV DOCKER_HOST=tcp://docker:2376

USER runner

RUN export SKIP_IPTABLES=1 \
    && curl -fsSL https://get.docker.com/rootless | sh \
    && /home/runner/bin/docker -v

ENTRYPOINT ["/bin/bash", "-c"]
CMD ["entrypoint-dood-rootless.sh"]
