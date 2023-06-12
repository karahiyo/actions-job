FROM ubuntu:22.04

ARG RUNNER_VERSION=2.304.0

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get -y update \
    && apt-get install -y \
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
    uidmap

ARG RUNNER_USER_UID=1001
RUN adduser --disabled-password --gecos "" --uid $RUNNER_USER_UID runner

ENV HOME=/home/runner

## Set-up subuid and subgid so that "--userns-remap=default" works
RUN addgroup --system dockremap \
    && adduser --system --ingroup dockremap dockremap \
    && echo 'dockremap:165536:65536' >> /etc/subuid \
    && echo 'dockremap:165536:65536' >> /etc/subgid

ENV RUNNER_ASSETS_DIR=/runnertmp
RUN mkdir -p "${RUNNER_ASSETS_DIR}" \
    && cd "${RUNNER_ASSETS_DIR}" \
    && curl -sfLo runner.tar.gz -L https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz \
    && tar xzf ./runner.tar.gz \
    && rm ./runner.tar.gz \
    && ./bin/installdependencies.sh

# Make the rootless runner directory executable
RUN mkdir /run/user/1000 \
    && chown runner:runner /run/user/1000 \
    && chmod a+x /run/user/1000

COPY entrypoint-dind-rootless.sh startup.sh /usr/bin/
RUN chmod +x /usr/bin/entrypoint-dind-rootless.sh /usr/bin/startup.sh

ENV PATH="/home/runner/bin:${PATH}"
ENV ImageOS=ubuntu22
ENV XDG_RUNTIME_DIR=/run/user/1000
#ENV XDG_RUNTIME_DIR=/home/runner/.docker/run
ENV DOCKER_HOST=unix:///run/user/1000/docker.sock
#ENV DOCKER_HOST=unix:///home/runner/.docker/run/docker.sock

# data directory for the docker daemon: /home/runner/.local/share/docker
# config directory for the docker daemon: /home/runner/.config/docker

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
