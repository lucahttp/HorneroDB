
### go run command
go run cmd/server/main.go

### useful command for development (compiles and runs)
cd .\web\ui\ && npm run build && cd ../.. && go run cmd/server/main.go

### docker compose command
docker compose up -d --no-deps --build hornerodb-server

### push to docker hub
docker build -t lukaneco/hornerodb:latest . && docker push lukaneco/hornerodb:latest