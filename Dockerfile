FROM scratch
ARG GOARCH=amd64

COPY bin/linux/${GOARCH}/arangodb /app/

EXPOSE 8528 

VOLUME /data

# Data directory 
ENV DATA_DIR=/data 

# Signal running in docker 
ENV RUNNING_IN_DOCKER=true

# Docker image containing arangod.
ENV DOCKER_IMAGE=arangodb/arangodb:latest

ENTRYPOINT ["/app/arangodb"]
