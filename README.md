# What Registrator Does

In Multiverse applications, each application registers itself with Eureka. Because Marvel has a load balancer placed in front of the application containers, we need to register the load balancer rather than each application.

Registrator monitors the running containers, determines the load balancer used for the application then registers the load balancer with Eureka and performs heartbeats.

With Multiverse you’ve probably seen cases where an instance will drop out of Eureka, this is usually because the application crashes and stops heartbeating. With Marvel we have multiple copies of registrator doing heartbeats. An application crashing will cause the container to get automatically replaced with no disruption to heartbeats.


If you are a consumer of this project in the open source community, and eureka support is useful for you, please feel free to contribute issues and PRs.

# Known Issues

We've seen 1 case of registrator hanging on an instance and not detecting new containers. In production with at least 3 instances of an application running this would only cause issues if all 3 registrator copies were hanging.

# Sumologic Logs

Prod: `_sourceCategory=app_registrator`
Thor `_index=thor_env _sourceCategory=THOR_app_registrator`


# Hudl Development Docs

There are docs on the hudl development process and tools for registrator [docs/dev/hudl.md](docs/dev/hudl.md). 


# Open Source Fork Information

This hudl version of registrator is a fork of the gliderlabs open source project, which was released under the MIT license (see further docs below). It has at this point been quite heavily customised for hudl, and has the main addition of a eureka support PR, which was never merged into the upsteam repository (see: https://github.com/gliderlabs/registrator/pull/360). Because of this, we've had some difficulty merging back improvements and changes. It should be possible to merge in upsteam changes, but there is some work involved to switch to using the new dependency management that the upsteam has adopted.

# Open Source Registrator Documentation

Service registry bridge for Docker.

[![Circle CI](https://circleci.com/gh/gliderlabs/registrator.png?style=shield)](https://circleci.com/gh/gliderlabs/registrator)
[![Docker pulls](https://img.shields.io/docker/pulls/gliderlabs/registrator.svg)](https://hub.docker.com/r/gliderlabs/registrator/)
[![IRC Channel](https://img.shields.io/badge/irc-%23gliderlabs-blue.svg)](https://kiwiirc.com/client/irc.freenode.net/#gliderlabs)
<br /><br />

Registrator automatically registers and deregisters services for any Docker
container by inspecting containers as they come online. Registrator
supports pluggable service registries, which currently includes
[Consul](http://www.consul.io/), [etcd](https://github.com/coreos/etcd) and
[SkyDNS 2](https://github.com/skynetservices/skydns/).

Full documentation available at http://gliderlabs.com/registrator

## Getting Registrator

Get the latest release, master, or any version of Registrator via [Docker Hub](https://registry.hub.docker.com/u/gliderlabs/registrator/):

	$ docker pull gliderlabs/registrator:latest

Latest tag always points to the latest release. There is also a `:master` tag
and version tags to pin to specific releases.

## Using Registrator

The quickest way to see Registrator in action is our
[Quickstart](https://gliderlabs.com/registrator/latest/user/quickstart)
tutorial. Otherwise, jump to the [Run
Reference](https://gliderlabs.com/registrator/latest/user/run) in the User
Guide. Typically, running Registrator looks like this:

    $ docker run -d \
        --name=registrator \
        --net=host \
        --volume=/var/run/docker.sock:/tmp/docker.sock \
        gliderlabs/registrator:latest \
          consul://localhost:8500

## Contributing

Pull requests are welcome! We recommend getting feedback before starting by
opening a [GitHub issue](https://github.com/gliderlabs/registrator/issues) or
discussing in [Slack](http://glider-slackin.herokuapp.com/).

Also check out our Developer Guide on [Contributing
Backends](https://gliderlabs.com/registrator/latest/dev/backends) and [Staging
Releases](https://gliderlabs.com/registrator/latest/dev/releases).

## Sponsors and Thanks

Big thanks to Weave for sponsoring, Michael Crosby for
[skydock](https://github.com/crosbymichael/skydock), and the Consul mailing list
for inspiration.

For a full list of sponsors, see
[SPONSORS](https://github.com/gliderlabs/registrator/blob/master/SPONSORS).

## License

MIT

<img src="https://ga-beacon.appspot.com/UA-58928488-2/registrator/readme?pixel" />
