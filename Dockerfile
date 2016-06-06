FROM fedora

#create working directory
RUN mkdir -p /usr/src/app/
WORKDIR /usr/src/app/
ENV WORKDIR /usr/src/app/

# Install dependencies
RUN dnf install -y bzip2 curl tar
RUN dnf install -y freetype fontconfig ImageMagick

# Install phantomjs
RUN curl -O -L https://bitbucket.org/ariya/phantomjs/downloads/phantomjs-1.9.8-linux-x86_64.tar.bz2
RUN bunzip2 phantomjs-1.9.8-linux-x86_64.tar.bz2
RUN tar -xvf phantomjs-1.9.8-linux-x86_64.tar
RUN ln -s /usr/src/app/phantomjs-1.9.8-linux-x86_64/bin/phantomjs /bin/phantomjs

COPY app $WORKDIR
COPY ack $WORKDIR

EXPOSE 8080

CMD ack
