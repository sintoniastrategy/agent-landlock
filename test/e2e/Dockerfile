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

COPY docker-entrypoint.sh /test/e2e/run-all.sh
COPY tests /test/e2e/tests
RUN chmod 0755 /test/e2e/run-all.sh /test/e2e/tests/*.sh

USER user1
WORKDIR /home/user1
ENTRYPOINT ["/test/e2e/run-all.sh"]
