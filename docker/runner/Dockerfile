FROM ubuntu:22.04

ARG RUNNER_VERSION=2.304.0

# Use 1001 and 121 for compatibility with GitHub-hosted runners
ARG RUNNER_UID=1000
ARG DOCKER_GID=1001

RUN apt-get -y update \
    && apt-get -y install curl expect jq

RUN adduser --disabled-password --gecos "" --uid $RUNNER_UID runner \
    && groupadd docker --gid $DOCKER_GID \
    && usermod -aG sudo runner \
    && usermod -aG docker runner \
    && echo "%sudo   ALL=(ALL:ALL) NOPASSWD:ALL" > /etc/sudoers \
    && echo "Defaults env_keep += \"DEBIAN_FRONTEND\"" >> /etc/sudoers

WORKDIR /actions-runner

RUN curl -sfLo runner.tar.gz -L https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz \
  && tar xzf ./runner.tar.gz \
  && rm ./runner.tar.gz \
  && ./bin/installdependencies.sh

COPY entrypoint.sh .

USER runner

ENTRYPOINT ["/bin/bash", "-c"]
CMD ["./entrypoint.sh"]
