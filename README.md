# P2P
## Добавление нового узла

```commandline
    docker network create --subnet=172.18.0.0/16 p2p_net
    docker-compose up --build
    docker-compose -f docker.compose.new-peers.yml up --build
```