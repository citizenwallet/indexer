FROM golang:1.21.6-alpine AS builder

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# create build folder
RUN mkdir -p /cw-build

# create binaries folder
RUN mkdir -p /cw

# move into build folder
WORKDIR /cw-build

# copy all go files into our container
COPY . .

# copy the .env file into our binaries folder
COPY .env /cw/.env
COPY firebase.json /cw/firebase.json

# install all plugins
RUN go mod download

# build
RUN go build -o /cw/main ./cmd/node/main.go

# clean up container
RUN rm -rf /cw-build

# Build a small image
FROM golang:1.21.6-alpine

RUN mkdir -p /cw

COPY --from=builder /cw/main /cw

COPY --from=builder /cw/.env /cw
COPY --from=builder /cw/firebase.json /cw

# define the command to be run on launch
ENTRYPOINT ["/cw/main"]

CMD ["-env", "/cw/.env", "-fbpath", "/cw/firebase.json"]