#!/bin/bash
LOG_FILE=/logs/registrator-$(hostname).log
exec > >(tee -a ${LOG_FILE} )
exec 2> >(tee -a ${LOG_FILE} >&2)

# Startup registrator and capture log output
# Needs to run inside the container

CMD="/bin/registrator $@"
echo Running: $CMD
$CMD

