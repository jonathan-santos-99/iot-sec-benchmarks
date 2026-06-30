#!/usr/bin/bash

CONTAINER_NAME="${1:-}"
USERNAME="${2:-}"
PASSWORD="${3:-}"

if [[ -z "$CONTAINER_NAME" || -z "$USERNAME" || -z "$PASSWORD" ]]; then
	echo "Usage: $0 <container name> <user> <password>"
	exit 1
fi

docker exec "$CONTAINER_NAME" touch /mosquitto/config/pwfile
docker exec "$CONTAINER_NAME" chmod 0700 /mosquitto/config/pwfile
docker exec "$CONTAINER_NAME" mosquitto_passwd -b -c /mosquitto/config/pwfile "$USERNAME" "$PASSWORD"
