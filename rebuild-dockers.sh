#!/bin/sh

docker build -t 127.0.0.1:5000/nginx ./nginx/
docker build -f accounter.dockerfile -t 127.0.0.1:5000/accounter .
docker build -f operator.dockerfile -t 127.0.0.1:5000/operator .
docker build -f pointer.dockerfile -t 127.0.0.1:5000/pointer .
docker build -f pusher.dockerfile -t 127.0.0.1:5000/pusher .
docker build -f weber.dockerfile -t 127.0.0.1:5000/weber .

docker push 127.0.0.1:5000/nginx
docker push 127.0.0.1:5000/accounter
docker push 127.0.0.1:5000/operator
docker push 127.0.0.1:5000/pointer
docker push 127.0.0.1:5000/pusher
docker push 127.0.0.1:5000/weber

docker stack deploy --compose-file docker-compose.yaml oper