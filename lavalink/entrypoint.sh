#!/bin/sh
set -e

PORT="${PORT:-2333}"
PASSWORD="${LAVALINK_PASSWORD:-youshallnotpass}"

sed "s/__PORT__/${PORT}/g; s/__PASSWORD__/${PASSWORD}/g" \
  /opt/Lavalink/application.yml.template > /opt/Lavalink/application.yml

exec java -jar /opt/Lavalink/Lavalink.jar
