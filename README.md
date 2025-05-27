# P2P
## Добавление нового узла

```commandline
    docker network create --subnet=172.18.0.0/16 p2p_net
    docker network create --subnet=172.20.0.0/16 presentation_net
    docker-compose up --build
    docker-compose -f docker.compose.new-peers.yml up --build
```

```
MATCH (a:Agent)-[r]->(b:Agent)
WHERE type(r) IN ['IS_HUB_FOR', 'IS_ABONENT_FOR']
RETURN a, r, b
```