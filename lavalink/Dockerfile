FROM ghcr.io/lavalink-devs/lavalink:4.2.2

USER root

COPY application.yml.template /opt/Lavalink/application.yml.template
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV _JAVA_OPTIONS="-Xmx512M"

WORKDIR /opt/Lavalink

ENTRYPOINT ["/entrypoint.sh"]
