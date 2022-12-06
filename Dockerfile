# To build this image:
#
#   docker build                                      \
#          [--build-arg GolangVersion=<X.YY.Z>]       \
#          [--no-cache]                               \
#          [-t <repository>[:<tag>]]                  .
#
#   Notes:
#     --build-arg GolangVersion:
#       1) identifies Golang version
#       2) default specified in ARG GolangVersion line
#     --no-cache:
#       1) tells Docker to ignore cached images that might be stale
#       2) useful due to Docker not understanding changes to build-args
#       3) useful due to Docker not understanding changes to context dir
#     -t:
#       1) provides a name REPOSITORY:TAG for the built image
#       2) if no tag is specified, TAG will be "latest"
#       3) if no repository is specified, only the IMAGE ID will identify the built image
#
# To run the resultant image:
#
#   docker run                                          \
#          [-d|--detach]                                \
#          [-it]                                        \
#          [--rm]                                       \
#          --cap-add SYS_ADMIN                          \
#          --device /dev/fuse                           \
#          --mount src="$(pwd)",target="/src",type=bind \
#          <image id>|<repository>[:<tag>]
#
#   Notes:
#     -d|--detach:  tells Docker to detach from running container 
#     -it:          tells Docker to run container interactively
#     --rm:         tells Docker to destroy container upon exit
#     --cap-add:    tells Docker to grant the SYS_ADMIN capability to the container (needed for FUSE)
#     --device:     tells Docker to expose the /dev/fuse device file to the container (needed for FUSE)
#     --privileged: tells Docker to, among other things, grant access to /dev/fuse
#     --mount:
#       1) bind mounts the context into /src in the container
#       2) /src will be a read-write'able equivalent to the context dir
#
# To launch an "sh" shell in the running container:
#
#   docker exec -it {<container id>|<name>} sh
#
#   Notes:
#     -it: tells Docker to run the command interactively

FROM alpine:3.17
ARG GolangVersion=1.19.4
RUN apk add --no-cache bind-tools   \
                       curl         \
                       fuse         \
                       gcc          \
                       git          \
                       jq           \
                       libc-dev     \
                       libc6-compat \
                       make         \
                       tar
ENV GolangBasename "go${GolangVersion}.linux-amd64.tar.gz"
ENV GolangURL      "https://golang.org/dl/${GolangBasename}"
WORKDIR /tmp
RUN wget -nv $GolangURL
RUN tar -C /usr/local -xzf $GolangBasename
ENV PATH $PATH:/usr/local/go/bin
VOLUME /src
WORKDIR /src
RUN git config --global --add safe.directory /src
