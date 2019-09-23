
# How Registrator Works

Refer to [the diagram](./diagram.md) for some more information on how eureka registrations happen locally and on ECS.

# Running Locally

Registrator has a Makefile to help with testing locally.  It will start a eureka container and the current registrator version pointing to that. 

It also has a docker-compose.yml to assist with testing. That will run a eureka container locally, as well as a dummy container to register. To start it, just run:

`docker-compose up --build`

## Go Setup

1. Make sure you have go installed.  You will need a 1.7.x or 1.8.x release.


###  Mac (with homebrew):

`brew install go`


### Windows:

Install from https://golang.org/dl/


2. Go is a little bit fussy about how it finds dependencies, this is a simpler way to resolve that problem (though there may be others).

    Go to the directory you wish to use as the src root in your shell.  Go will make a bit of a mess of this, so it could be a directory underneath where you usually check out things, e.g

    `c:\src\go` or `/Users/my.name/src/go`

    **Mac**:

    ```
    export GOPATH=$(pwd)
    echo "export GOPATH=$(pwd)" >> ~/.bashrc
    ```

    You might want to put it in `~/.zshrc` instead if you use zsh.

    **Windows**:

    a. Right Click on My Computer / This PC and choose Properties
    b. Click Advanced System Properties > Environment Variables
    c. Add a new system variable, looking something like this:

    [Product Stack Tribe > Making Changes to Registrator > image2017-6-20_9-58-58.png]

3. Start a new shell, and check out the code in the usual way.  The best place to do this to is by creating a directory structure like this:

    **Mac**

    ```
    mkdir -p $GOPATH/github.com/gliderlabs
    cd $GOPATH/github.com/gliderlabs
    ```

    **Windows** 

    ```
    mkdir $env:GOPATH/github.com/gliderlabs
    cd $env:GOPATH/github.com/gliderlabs
    ```

    **Both**

    `git clone git@github.com:hudl/registrator.git`



## Run Registrator

Run make dev in the registrator directory.

This will start a local eureka instance running at http://localhost:8090 and compile and run registrator in a docker container, pointing at that.


## Making Changes

Be cautious about testing changes to eureka registration code in the normal thor envrionment. Problems with eureka registrations can potentially bring down the nginx servers and stop everyone being able to access any test branches. It's best to test on an isolated eureka server before rolling out to thor. There's a testsidecar environment for this purpose.


To test changes to registrator, use the following process:

1. Create a new local branch in git, as normal.
2. Test locally where possible.  Some changes, such as anything to do with AWS/ALBs it won't be easy to test in isolation.  One possible option is to create a new go project in another directory and test isolated functions there.  Another is to use the AWS SDK unit testing interfaces - see: 
3. Git push your branch.
4. Go to teamcity and make sure your branch builds.  It will also post the result of the build in the #deploy-testing slack channel.
5. Deploy to the testsidecar envrionment, with something like this:
   ```
   /deploy push registrator ORCH-NoMoreThrottling testsidecar
   ```
   
6. This environment has an isolated eureka server, running at  http://registrator-testing.thorhudl.com:8080/eureka/ - Thor is considered a production like environment for sidecar changes.  Any problems with eureka updates can potentially break lots of people's environments, which is why we have an isolated testsidecar configuration to test on instead.
7. Check sumologic output from the sourceCategory=THOR_app_registrator
8. Test your changes
9. Merge to master branch when ready to release.
10. Then continue to deploy in other environments starting with thor.  

## Tests

There are a few unit tests you can run with:
```
go test ./...
```

## Docker Distribution / Release


There is a teamcity job to build registrator correctly and create the appropriate service defition files on S3.

The builds should happen automatically when you push a new branch, but it's currently on a polling interval, so could take a while.  You can also start one manually in the usual way.

The teamcity job is responsible for creating hudl service definition (HSD) files, which the marvel deploy tool can then utilise to deploy.


## Publishing a New Registrator Version

When a master build is completed successfully, registrator will automatically be published to the latest tag in ECR, along with a versioned master tag. This will automatically be used by marvel local dev experience. However, a new registrator version will only be checked for once per week. As such, you should test any registrator changes locally as well to ensure they work correctly, as the release to local dev is delayed and will not happen immediately.


## Eureka Metadata

In order to be backward compatible with the existing multiverse, registrator sets data it posts to eureka match the existing services. The following is a list of things that need to be specified and included.  

Where Field begins with metadata these are usually specified via docker image labels, or at runtime as envrionment variables.

| Field                                     | Used By                          | Value for Local Dev       | Value in ECS                                                                                                |
|-------------------------------------------|----------------------------------|---------------------------|-------------------------------------------------------------------------------------------------------------|
| hostName                                  | registrator, others?             | Set to VPN IP             | Hostname of ALB for service                                                                                 |
| ipAddr                                    | bifrost                          | Set to VPN IP             | Set to host IP                                                                                              |
| vipAddress                                | tests                            | Set to VPN IP             | Set to host IP                                                                                              |
| metadata.hudl.version                     | warpdrive, thor testing, bifrost | -                         | Embedded in container image from build time. Used for bifrost routing.                                      |
| metadata.branch                           | alyx3, nginx, bifrost            | Local branch              | Embedded in container image from build time. Used for nginx routing.                                        |
| metadata.deployNumber                     | alyx3                            | -                         | Use env var when creating task definition, if still required                                                |
| metadata.hudl.routes                      | bifrost, nginx                   | Taken from service config | Embedded in container image from build time                                                                 |
| metadata.aws-instance-id                  | Informational                    | -                         | Actual instance ID, except for load balancer entries.                                                       |
| metadata.thor-communal                    | Communals / Thor Master          | -                         | Required for multiverse service bifrost calls. Not required by marvel services.                             |
| metadata.container-id                     | Informational                    | Docker container ID       | -                                                                                                           |
| metadata.container-name                   | Informational                    | Docker container name     | -                                                                                                           |
| metadata.is-container                     | Registrator                      | TRUE                      | -                                                                                                           |
| metadata.elbv2-endpoint                   | Registrator                      | -                         | ALB endpoint string                                                                                         |
| metadata.has-elbv2                        | Registrator                      | -                         | True (used by registrator itself)                                                                           |
| metadata.aws-instance-id                  | Informational                    | -                         | Generally this is empty, as ALBs are registered. If you turn of ALB registration it would be populated.     |
| dataCenterInfo.metadata.local-hostname    | Informational                    | -                         | Private (VPC) IP/Name mapping (e.g. ip-xx-xx-xx-xx.ec2.internal)                                            |
| dataCenterInfo.metadata.availability-zone | Informational                    | -                         | Actual AZ                                                                                                   |
| dataCenterInfo.metadata.instance-id       | Informational                    | -                         | Unique ID given by hostname_port - not AWS instance ID                                                      |
| dataCenterInfo.metadata.public-ipv4       | Informational                    | -                         | Instance public IP (if available)                                                                           |
| dataCenterInfo.metadata.public-hostname   | Informational                    | -                         | Instance public hostname - e.g. ec2-xx-xx-xx-xx.compute-1.amazonaws.com (if available)                      |
| dataCenterInfo.metadata.local-ipv4        | Informational                    | -                         | Instance private (VPC) IP address                                                                           |
| dataCenterInfo.metadata.hostname          | Informational                    | -                         | Same as local-hostname                                                                                      |


## Container Labels / Flags

To add some of the custom metadata we need, these are some of the docker run environment variable options (or labels on the container).  A full list is included here  Also see http://gliderlabs.com/registrator/latest/user/services/#container-overrides

Most of these options are added automatically by the marvel CLI tooling.  Usually the appropriate place to add additional ones for non-marvel services would be the service-config files in that particular repo.

Environment variables take precedence over container labels of the same name, see [here](https://github.com/hudl/registrator/blob/e28f259ce8c954d52571693f9441d0bcf1250896/bridge/util.go#L140).

## Eureka Heartbeat

Registrator sends a eureka heartbeat by calling the Refresh function for the eureka service plugin.  It does this on the interval specified by the `--ttl-refresh` startup option.
Risk

If the registrator goes down on a host, all the apps running on that host will eventually disappear from eureka. Depending on the situation, this could be a double edged sword. If a host dies unexpectedly, as a consequence all apps on it will disappear from eureka, which is somewhat what we want. I say "somewhat" because the disappearance is not immediate, they still wait till the heartbeat fails.  With using Application Load Balancers, the risk is lessened, because losing a host will just mean some of the heartbeats stopping for the same ALB.  So it's potentially possible to lose registrator on most of the hosts, but still have the containers on the same hosts continue to receive traffic (as they are being served by the same Application Load Balancer as the rest of the containers).

If the registrator simply panics, then depending on whether this is a reoccurring problem, the risk could be greater.  If the process crashes for any reason, ECS will normally start a new container automatically.

The biggest risks with registrator are to do with code changes, where they affect eureka registrations.  


## Known Issues

There is a known problem where if an ELBvs (ALB) registration stays in eureka longer than it should do, it can stop nginx reloads from happening correctly.  This is because once an ALB is deleted, it's hostname record stops resolving.  Nginx tries to resolve the upstream hostname when it reloads, and this stops the reload from occurring correctly.  

This happens in the following circumstance:

1. A thor branch is deleted using the unthor command or manually, and the load balancer is also deleted.
2. There is a delay in the ALB dropping out of eureka (should take 35s from the last heartbeat in the latest registrator code)
3. Nginx tries to reload the configuation in the period between the ALB hostname record being deleted, and the ALB dropping from eureka.

In practice, this issue usually resolves itself quickly because eventually the ALB record drops from eureka, and then nginx is able to reload again.

There was a 2nd possible cause of this, where the ALB eureka record keeps receiving a heartbeat after it has been deleted.  The known data race causes of this problem in registrator have been fixed, however it is possible it could reoccur in a future circumstance.  One such similar issue is a hanging container on an ECS server.  If a container never correctly exits, it will keep having heartbeats sent on it's behalf.  As such, monitoring for the nginx error in the future is a good idea.

