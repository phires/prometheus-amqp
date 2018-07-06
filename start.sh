#!/bin/bash
set -e

echo Starting main
if [ ! -f $FILTERFILE ]; then
    touch $FILTERFILE
fi

/main --log-only --amqp-address $AMQP_ADDRESS --amqp-queue $AMQP_QUEUE --filter-file $FILTERFILE