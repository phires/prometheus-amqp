FROM ubuntu:latest

RUN apt-get update \
    && apt-get install -y ca-certificates 

COPY main /
COPY start.sh /
RUN chmod +x /main \
    && chmod +x /start.sh
EXPOSE 24282
WORKDIR /
ENTRYPOINT [ "/start.sh" ]