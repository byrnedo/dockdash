FROM golang:1.18 as build

WORKDIR /app

ENV CGO_ENABLED 0
ENV GOOS linux

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -installsuffix cgo -o dockdash .

FROM scratch

COPY --from=build /app/dockdash /dockdash

CMD ["/dockdash"]
