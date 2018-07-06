#!/bin/bash
set -e

echo Starting main
if [ ! -f $FILTERFILE ]; then
    touch $FILTERFILE
fi

if [[ $LOGONLY = "true" ]]
then
    /main --log-only --amqp-address $AMQP_ADDRESS --amqp-queue $AMQP_QUEUE --filter-file $FILTERFILE
else
    /main --amqp-address $AMQP_ADDRESS --amqp-queue $AMQP_QUEUE --filter-file $FILTERFILE
fi