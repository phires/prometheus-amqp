#!/bin/bash
set -e

echo Starting main
/main --amqp-address $AMQP_ADDRESS --amqp-queue $AMQP_QUEUE