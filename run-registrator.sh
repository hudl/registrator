#!/bin/bash
DATE=$(date +%d%m%Y-%H%M%S)
LOG_FILE=/logs/registrator-$(hostname)-$DATE.log
exec > >(tee -a ${LOG_FILE} )
exec 2> >(tee -a ${LOG_FILE} >&2)

# Startup registrator and capture log output
# Needs to run inside the container

CMD="/bin/registrator $@"
$CMD

