# Build stage: compile into a single static binary
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -o /out/hello .

# Run stage: distroless base image, small and fast to start, good for kind load in a live demo
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/hello /hello
EXPOSE 8080
ENTRYPOINT ["/hello"]
