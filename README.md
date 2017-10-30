ding - build server

Ding builds your software projects from git repositories.
You will typically configure a "post-receive" hook on your git server to tell ding to start a build.
Ding will start the compile, run tests, and make resulting binaries/files.  Such results can be promoted to a release.

All command output of a build is kept around. If builds/tests go wrong, you can look at the output.
Build dirs are kept around for some time, and garbage collected automatically when you have more than 10 builds, or after 2 weeks.

Dingkick can be used in a git hook to signal that a build should start.


# Todo

- make this work with github repos, perhaps bitbucket as well.  requires having a public face. that requires adding auth...
- get live updates during builds using SSE or something similar
- do some extra steps? like coverage checking, and displaying the results.
- add docs, at least a page describing the design.

## Maybe
- clone & checkout also through shell script?
- add shell script to cleanup after a build? eg dropping a database.
- add shell file that can be sourced in the other scripts, for common code?
- timestamps in output lines?
- security: can we run the builds as a separate user? how to make sure the build cannot touch files outside of its own directory?
- be more helpful for building go projects by putting the checkout dir in place that can be used as gopath, in what is probably the right path (git.example.com:yourname.git -> git.example.com/yourname)?
- compress released files with gzip and serve them gzipped if possible


# Design

## steps
- clone
- checkout
- build
- test
- release
- success

## directory layout
- config/<repo>/
	build.sh
	test.sh
	release.sh
- build/<repo>/<buildID>/
	checkout/
	scripts/
		build.sh
		test.sh
		release.sh
	output/
		{build,test,release}.{stdout,stderr,output,nsec}
- release/<repo>/<buildID>
	filename
