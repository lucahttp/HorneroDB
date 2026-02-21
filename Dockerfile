FROM golang:1.25-bookworm

WORKDIR /usr/src/app

# Copy files that list dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the server tool
RUN go build -v -o /usr/local/bin/hornerodb ./cmd/server

# Run the tool
CMD ["hornerodb"]