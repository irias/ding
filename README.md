ding - build server

Ding builds your software projects from git repositories.
You will typically configure a "post-receive" hook on your git server to tell ding to start a build.
Ding will start the compile, run tests, and make release binaries/files.
All command output is kept around. If builds/tests go wrong, you can look at the output.
Build dirs and release artefacts are kept around for some time, and garbage collected automatically after some time. However, you can mark binaries as released, so they will never be removed.
Ding also lets you (makes you) write some scripts to set configuration files and requirements such as a database.

Dingkick can be used in a git hook to signal that a build should start.

# Todo

- implement button to "build latest version of branch"?
- clone & checkout also through shell script?
- add shell script to cleanup after a build. eg dropping a database.
- timestamps in output lines?
- test with more repositories
- security: can (should) we run the builds as a separate user? how to make sure the build cannot touch files outside of its own directory?
- how to clean up builds? we currently keep all the checkouts as is. should remove after certain time or certain number of builds per repo, so we don't clog the disk.  we should also be able to remove the build dirs while keeping the resulting binaries (and perhaps store the resulting binaries with gzip, can add up with these big go binaries).
- button to restart a build with same scripts
- make this work with github repos, perhaps bitbucket as well.  requires having a public face. that requires adding auth...
- get live updates during builds using SSE or something similar
- do some extra steps? like coverage checking, and displaying the results.

# steps
- clone
- checkout
- build
- test
- release
- success

# files
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
- release/<repo>/
	filename
