# Ding - build server for developers

Ding builds your software projects from git/mercurial/etc repositories.

You will typically configure a "post-receive" (web)hook on your git
server to tell ding to start a build.

Ding will start the compile, run tests, and make resulting
binaries/files.  Successful results can be promoted to a release.

All command output of a build is kept around. If builds/tests go
wrong, you can look at the output.

Build dirs are kept around for some time, and garbage collected
automatically when you have more than 10 builds, or after 2 weeks.

Dingkick is a tool you can use in a git hook to signal that a build
should start. Github and bitbucket webhooks are also supported.

Ding provides a web API at /ding/, open it once you've got it
installed. It includes an option to get real-time updates to builds
and repositories.

See INSTALL.md for instructions on how to install. "ding help"
prints these instructions as well. And you can read them at /INSTALL.md
when you've got ding running.


# Features

- Self-hosted build server. Keep control over your code and builds!

- Simple. Get started quickly, experience the power of simplicity,
use your existing skills, avoid the burden of complex systems.

- Isolated builds, each build starts under its own unix user id:
extremely fast, and builds can't interfere with each other.

- (Web) API for all functionality, which is what the web app is
using.


# Non-features

We do _NOT_:

- do deployments: Different task, different software. Ding exports
released files which can be picked up by deployment tools.

- want to be all things to everybody: Ding does not integrate with
every VCS/SCM, does not have a plugin infrastructure, and does not
hold your hand.

- use docker images: Ding assumes you create self-contained programs,
such as statically linked Go binaries or JVM .jar files. If you
need other services, say a database server, just configure it when
setting up your repository in Ding. If you need certain OS dependencies
installed, first try to get rid of those dependencies. If that isn't
an option, install the dependencies on the build server.

- call ourselves "continuous integration" or CI server. Mostly
because that term doesn't seem to be describing what we do.


# FAQ

Q: Why are there no questions here?
A: Because you didn't ask them yet. Let us know your questions! Please?


# Developing

You obviously need a Go compiler.
But you'll also need:
- python (v2) to build the frontend files
- jshint through npm and node to check the JavaScript code
- sass through gem and ruby to create CSS files

See the Makefile on how to build and run this.


# Todo


## Maybe
- merge the "checkout" step into "clone"? already not always necessary, and it's not a big enough step.
- allow configuring a cleanup script, that is run when a builddir is removed. eg for dropping a database that was created in build.sh.
- authentication. we currently expect ding to be installed on a private network, where everyone with access is trusted.
- read some file from $HOME after a build and show it in build overviews? eg for code coverage, or whatever. easy & extensible.
- provide access to the builddir from the previous build, eg to copy dependencies. or perhaps we could also do a faster clone ourselves.
- implement timeouts for builds.  no output for X minutes -> kill.
- do more? like reading test coverage somewhere and displaying that
- add shell script to cleanup after a build? eg dropping a database.
- timestamps in output lines?
- compress released files with gzip and serve them gzipped if possible
- more ways to send out notifications. eg webhook, telegram, slack.
- support cloning mercurial repo's? perhaps others.
- clone & checkout also through shell script?
- support for running builds on other instances (on other OS'es). maybe some day, if really needed.
