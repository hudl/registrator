#!/bin/bash

# Builds a Docker image named <registry>/dotnet-microservice-build:<TAG> from the
# adjacent Dockerfile.

# For CI (e.g. TeamCity) execution, pass in three arguments:
#     BRANCH: Branch being built
#     GIT_SHA1_7CHAR: 7-character Git SHA-1 for the commit being built.
#     BUILD_NUMBER: CI-controlled incrementing number
#
#    ./build-image.sh <BRANCH> <GIT_SHA1_7CHAR> <BUILD_NUMBER> [REGISTRY_URI]
#
#    # Example: ./build-image.sh MyTestBranch f3bb7a6 00245
#
# For local execution, pass a single argument to use a custom tag. You
# shouldn't really need to do this unless you're iterating on the Dockerfile
# itself. To use the build container image from ECR, you should interact with
# the dotnet-microservice Tools Package and call `dotnet marvel build`.
#
#    ./build-image.sh <TAG> [REGISTRY_URI]
#
#    # Example: ./build-image.sh my-local-test-image
#
# Both usages support an optional REGISTRY_URI as the last parameter. If set,
# the value will be used when tagging the built image, and the image will be
# pushed to the repository. This script assumes ECR, and will attempt an
# `aws ecr --get-login` first. Omit any leading or trailing slashes on the URI.
#
# Example REGISTRY_URI: 761584570493.dkr.ecr.us-east-1.amazonaws.com
#
# Note: If you look at ECR, it will list the "Repository URI", which is a
# concatenation of the registry URI and repository name: <registry>/<repository>.
# We only want the registry component as the REGISTRY_URI argument here. The
# repository component is defined by this script (as $REPOSITORY, below). when
# pushing, we assume that the repository in ECR matching the one here has already
# been created; if not, the push will fail.

# Input will be sanitized:
# - Slash "/" and dot "." characters will be replaced with underscore "_" characters.
# - Any prefixes of "refs/heads/" will be removed.
# - The inputted GIT SHA-1 will be truncated to the first 7 characters if longer.

# These values are not passed in as arguments.
REPOSITORY=registrator
BUILD_DATE=`date +%Y%m%d-%H%M`

# 1 to publish to the registry, 0 to skip publishing; may be set to 1 below.
PUBLISH=0

function sanitize
{
    local input=$1

    # Since "/" is significant for Docker image naming, we need to:
    # - remove refs/heads/ as a prefix from any incoming branch names
    # - replace any additional slashes with an underscore
    input=${input#refs/heads/}
    input=${input//\//_}

    # Since "." is our delimiter for tags, let's replace those with underscores too.
    input=${input//./_}

    echo "$input"
}

function truncate_git_sha1_7chars
{
    local input=$1
    input=${input:0:7}

    echo "$input"
}

# One argument is a local build with a custom tag.
if [ "$#" -eq 1 ] || [ "$#" -eq 2 ]; then
    CUSTOM_TAG=$(sanitize $1)

    if [ "$#" -eq 2 ]; then
        PUBLISH=1
        REGISTRY_URI=$2
        REPOSITORY=$REGISTRY_URI/$REPOSITORY
    fi

    REPOSITORY_AND_TAG=$REPOSITORY:$CUSTOM_TAG

# Three arguments are for official CI builds.
elif [ "$#" -eq 3 ] || [ "$#" -eq 4 ]; then
    # TODO since we use "." as a delimiter, should we strip/replace it from inputted argument values?
    # TODO also strip slashes
    BRANCH=$(sanitize $1) # TODO more aggressive sanitation here, since it's more beyond our control and unpredictable
    GIT_SHA1_7CHAR=$(truncate_git_sha1_7chars $(sanitize $2))
    BUILD_NUMBER=$(sanitize $3)

    if [ "$#" -eq 4 ]; then
        PUBLISH=1
        REGISTRY_URI=$4
        REPOSITORY=$REGISTRY_URI/$REPOSITORY
    fi

    # Branch and Build Date are first because they'll make it easy to quickly list and sort images by branch, then time.
    REPOSITORY_AND_TAG=$REPOSITORY:$BRANCH.$BUILD_DATE.$GIT_SHA1_7CHAR.$BUILD_NUMBER

# Any other argument count is invalid.
else
    echo "Invalid argument count, expecting one of:"
    echo "  Usage <CI build>:    $0 <BRANCH> <GIT_SHA1_7CHAR> <BUILD_NUMBER> [<REGISTRY_URI>]"
    echo "  Usage <local build>: $0 <TAG> [<REGISTRY_URI>]"
    exit 1
fi

echo "Building Docker image $REPOSITORY_AND_TAG"
docker build --pull --no-cache -t $REPOSITORY_AND_TAG .
BUILD_RESULT=$?

if [ $BUILD_RESULT -eq 0 ] && [ $PUBLISH -eq 1 ]; then
    echo "Publishing ${REPOSITORY_AND_TAG}"
    $(aws ecr get-login --region=us-east-1 --no-include-email) 2>&1

    # Log success message for build failure condition
    docker push $REPOSITORY_AND_TAG && echo "Publish succeeded."
    PUSH_RESULT=$?

    docker rmi $REPOSITORY_AND_TAG

    echo "${REPOSITORY_AND_TAG}" > PublishedImageUrl.txt
    exit $PUSH_RESULT
fi

if [ $BUILD_RESULT -ne 0 ]; then
    exit $BUILD_RESULT
fi
