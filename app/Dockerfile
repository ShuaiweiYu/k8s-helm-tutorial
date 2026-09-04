# Build stage: compile into a single static binary
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
# VERSION / CRASH_ON_START are how one source file becomes the three image
# tags used in the 47-57 min lifecycle demo — see ../05-lifecycle/build-images.sh.
ARG VERSION=v1
ARG CRASH_ON_START=false
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION} -X main.crashOnStart=${CRASH_ON_START}" -o /out/hello .

# Run stage: distroless base image, small and fast to start, good for kind load in a live demo
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/hello /hello
EXPOSE 8080
ENTRYPOINT ["/hello"]
