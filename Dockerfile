FROM alpine:3.5

MAINTAINER zweizeichen@element-43.com

#
# Copy release to container and set command
#

# Add faster mirror and upgrade packages in base image, Erlang needs ncurses, NIF libstdc++
RUN printf "http://mirror.leaseweb.com/alpine/v3.5/main\nhttp://mirror.leaseweb.com/alpine/v3.5/community" > etc/apk/repositories && \
    apk update && \
    apk upgrade && \
    apk add zeromq ca-certificates && \
    rm -rf /var/cache/apk/*

# Copy build
COPY market-streamer market-streamer

CMD ["/market-streamer"]