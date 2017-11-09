ding - build server

Ding builds your software projects from git repositories.
You will typically configure a "post-receive" hook on your git server to tell ding to start a build.
Ding will start the compile, run tests, and make resulting binaries/files.  Such results can be promoted to a release.

All command output of a build is kept around. If builds/tests go wrong, you can look at the output.
Build dirs are kept around for some time, and garbage collected automatically when you have more than 10 builds, or after 2 weeks.

Dingkick can be used in a git hook to signal that a build should start. Github webhooks are also supported.

See INSTALL.md for instructions on how to install. "ding help" prints these instructions as well.


# Todo

- depend on fewer shell scripts
- check whether automatically cleaning builds works with cleaning dirs vs removing builds vs the releases
- explain how to stop listening for github webhooks
- for failed builds, show text for that block in red or something. on success, show a green success bar/block.
- automatically make up repo name based on origin (which we'll ask first)
- ask user to enter a "checkout dir", so we can work more easily with go


## Maybe
- do more? like reading test coverage somewhere and displaying that
- add shell script to cleanup after a build? eg dropping a database.
- timestamps in output lines?
- compress released files with gzip and serve them gzipped if possible
- more ways to send out notifications. eg webhook, telegram, slack.
- support cloning mercurial repo's? perhaps others.
- clone & checkout also through shell script?


# Design

## steps
- new
- clone
- checkout
- build
- success

## directory layout
data/
	build/<repo>/<buildID>/
		checkout/
		scripts/
			build.sh
		output/
			<step>.{stdout,stderr,output,nsec}
	release/<repo>/<buildID>
		filename
