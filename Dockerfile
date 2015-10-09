FROM       alpine:edge
MAINTAINER Johannes 'fish' Ziemke <fish@docker.com> (@discordianfish)
EXPOSE     9104

ENV  GOPATH /go
ENV APPPATH $GOPATH/src/github.com/elephanter/container-exporter
COPY . $APPPATH
RUN echo "http://dl-4.alpinelinux.org/alpine/edge/community" >> /etc/apk/repositories; apk add --update -t build-deps go git mercurial libc-dev gcc libgcc \
    && cd $APPPATH && go get -d && go build -o /bin/container-exporter \
    && apk del --purge build-deps && rm -rf $GOPATH

ENTRYPOINT [ "/bin/container-exporter" ]
