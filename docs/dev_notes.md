
### UI Development commands recommendation

export HORNERO_ENV=development && go run cmd/server/main.go
cd web/ui && npm run dev && cd ../..  

### useful command for "PROD BUILD" development (compiles and runs)
cd .\web\ui\ && npm run build && cd ../.. && go run cmd/server/main.go

### useful command for "docker compose Build" development  
docker compose up -d --no-deps --build hornerodb-server

### push to docker hub
docker build -t lukaneco/hornerodb:latest . && docker push lukaneco/hornerodb:latest