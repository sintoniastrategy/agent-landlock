FROM debian:trixie-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash \
        ca-certificates \
        coreutils \
        findutils \
        grep \
        passwd \
        procps \
        sed \
        util-linux \
    && useradd -m -s /bin/bash user1 \
    && rm -rf /var/lib/apt/lists/*

COPY docker-entrypoint.sh /e2e-agent-lsm/run-all.sh
COPY tests /e2e-agent-lsm/tests
RUN chmod 0755 /e2e-agent-lsm/run-all.sh /e2e-agent-lsm/tests/*.sh

USER user1
WORKDIR /home/user1
ENTRYPOINT ["/e2e-agent-lsm/run-all.sh"]
