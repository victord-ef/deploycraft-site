---
title: "Docker"
description: "Docker image, container, volume, network, and Compose commands for day-to-day development and operations."
icon: "🐳"
weight: 2
count: 50
tags: ["docker", "containers", "devops"]
---

## Images

```bash
docker build -t <name>:<tag> .
docker build -t <name>:<tag> -f Dockerfile.prod .
docker build --no-cache -t <name>:<tag> .
docker pull <image>:<tag>
docker push <registry>/<image>:<tag>
docker images
docker images --filter dangling=true
docker rmi <image>
docker rmi $(docker images -q --filter dangling=true)
docker tag <source> <target>
docker save <image> | gzip > image.tar.gz
docker load < image.tar.gz
docker history <image>
docker inspect <image>
```

## Containers

```bash
docker run -d --name <name> -p 8080:80 <image>
docker run -it --rm <image> /bin/bash
docker run -e ENV_VAR=value <image>
docker run -v /host/path:/container/path <image>
docker run --network <network> <image>
docker run --restart unless-stopped <image>
docker run --memory 512m --cpus 1 <image>
docker ps
docker ps -a
docker stop <container>
docker start <container>
docker restart <container>
docker rm <container>
docker rm -f <container>
docker rm $(docker ps -aq -f status=exited)
docker exec -it <container> /bin/bash
docker exec -it <container> sh -c "env"
docker logs <container>
docker logs -f <container>
docker logs --tail 100 <container>
docker inspect <container>
docker stats
docker stats --no-stream
docker top <container>
docker cp <container>:/path/file ./local/
docker cp ./local/file <container>:/path/
```

## Volumes

```bash
docker volume create <name>
docker volume ls
docker volume inspect <name>
docker volume rm <name>
docker volume prune
```

## Networks

```bash
docker network create <name>
docker network create --driver bridge <name>
docker network ls
docker network inspect <name>
docker network connect <network> <container>
docker network disconnect <network> <container>
docker network rm <name>
docker network prune
```

## Compose

```bash
docker compose up -d
docker compose up --build -d
docker compose down
docker compose down -v                  # remove volumes too
docker compose ps
docker compose logs -f
docker compose logs -f <service>
docker compose exec <service> sh
docker compose pull
docker compose restart <service>
docker compose scale <service>=3
docker compose config                   # validate and print resolved config
```

## Cleanup

```bash
docker system prune
docker system prune -a                  # includes unused images
docker system prune -a --volumes
docker image prune
docker container prune
docker volume prune
docker system df                        # disk usage breakdown
```
