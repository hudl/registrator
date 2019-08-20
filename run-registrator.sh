#!/bin/bash

# Startup registrator, logs will be written to console.

# Needs to run inside the container

CMD="/bin/registrator $@"
$CMD

