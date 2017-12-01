# Installing

You'll need an empty postgres database, and a config.json file like:

	{
		"showSherpaErrors": true,
		"printSherpaErrorStack": true,
		"database": "dbname=ding host=localhost user=ding password=secretpassword sslmode=disable connect_timeout=3 application_name=ding",
		"environment": {
			"GEM_PATH": "/home/ding/.gem/ruby/2.3.0",
			"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/home/ding/node_modules/.bin/:/home/ding/.gem/ruby/2.3.0/bin:/home/ding/toolchains/bin",
			"TOOLCHAINS": "/home/ding/toolchains"
		},
		"notify": {
			"name": "devops",
			"email": "devops@example.org"
		},
		"baseURL": "https://ding.example.org",
		"githubWebhookSecret": "very secret",
		"bitbucketWebhookSecret": "very secret but different",
		"run": ["/usr/bin/nice", "/usr/bin/timeout", "600"],
		"isolateBuilds": {
			"enabled": false,
			"dingUid": 1001,
			"dingGid": 1001,
			"uidStart": 10000,
			"uidEnd": 20000
		},
		"mail": {
			"enabled": false,
			"from": "info@example.org",
			"fromName": "Ding",
			"replyto": "",
			"replytoName": "",
			"smtpHost": "localhost",
			"smtpTls": true,
			"smtpPort": 587,
			"smtpUsername": "username",
			"smtpPassword": "secretpassword"
		}
	}

Then give the database initialization a try.
You'll use this for upgrades in future versions as well:

	ding upgrade config.json

And now with commit if the previous was successful:

	ding upgrade config.json commit


# Dependencies

Make sure you have git installed if you plan to build git repositories.
Or mercurial (hg), or any other VCS you want to use.


# Notifications

You probably want to enable email notifications for failed builds.
Configure a mail server, and set "mail", "enabled" to true.

Ding does not support other mechanisms to send notifications (like
outgoing webhooks, or IRC/telegram/slack/etc). Instead Ding has a
real-time streaming updates API that can be used for those purposes.


# Isolate builds

You should also isolate builds by running each build under a unique
user id (UID):

- Configure the "isolateBuild" section in your config file. "dingUid"
and "dingGid" are the id's that the ding webserver will run under.
"uidStart" (inclusive) and "uidEnd" (exclusive) denotes the range
of user id's that ding will assign to builds. Build commands use
"dingGid" as their gid. Make sure the UIDs don't overlap with regular
users.
- Start ding as root, with umask 027. The umask ensures the
unpriviledged ding process can read build results.

"Run as root? Are you crazy?" No. Ding isn't actually running all
its code with root priviledges. Early during startup, Ding forks
off a child process with dinguid/dinggid. That process handles all
HTTP requests. There is still a process running as root, but its
only purpose is:

1. Starting builds under a unique UID.
2. Managing files created by the unique UID, such as chown/remove them.

The processes communicate through a simple protocol over a unix
socket. This privilege separation technique is popularized by
OpenBSD.

Why not use "sudo"? Because it does not seem possible to add sudo
rules for ranges of UIDs.


# Post-receive hook on git repositories

If you are running your own git server, you need to install a
post-receive hook on your git repositories. Create an executable
file `.git/hooks/post-receive`. Example script:

```sh
	#!/bin/sh
	PATH=$PATH:$HOME/bin
	repo=$(basename $PWD | sed 's/\.git$//')
	while read oldrev newrev refname; do
	        case $refname in
	        refs/tags/*)
	                branch='master'
	                ding kick https://your-ding-server/ding/ "$repo" "$branch" "$newrev"
	                ;;
	        refs/heads/*)
	                branch=$(echo $refname | sed 's,^refs/heads/,,')
	                ding kick https://your-ding-server/ding/ "$repo" "$branch" "$newrev"
	                ;;
	        esac
	done
```

That will kick off builds for every incoming commit. For incoming
tags, it will rebuild the master branch. This assumes you tag only
on your master branch and bake tags into release version numbers.


# Github and bitbucket webhooks for push events

Ding supports starting builds on pushes to github or bitbucket
repositories.  Start ding with the -listenwebhooks flag and set
"githubWebhookSecret" and/or "bitbucketWebhookSecret" in the config
file.

You'll need to configure a "webhook" for your repositories.

For github:

- Make a URL that points to your server, with path /github/<repoName>.
- Select "application/json" as event type - send only "push" events
(default at the time of writing) - set the same secret as in the
config file.

For bitbucket:

- Make a URL that points to your server, with path
/bitbucket/<repoName>/<bitbucketWebhookSecret>. Bitbucket does not
sign its requests, so the authentication is in the URL.

If you don't want to listen for webhook events, pass an empty string
to the -listenwebhook flag.


# Webserver configuration

You might want to run a HTTP proxy in front of Ding. Nginx is a
popular choice. Here is an example config file that keeps server-sent
events working:

	server {
		listen 10.0.0.1:80;
		server_name ding-internal.example.com;

		location / {
			include /etc/nginx/proxy_params;
			proxy_pass http://127.0.0.1:6084;
		}
		location = /events {
			include /etc/nginx/proxy_params;
			proxy_pass http://127.0.0.1:6084;
			proxy_buffering off;
			proxy_cache off;
			proxy_set_header Connection '';
			proxy_http_version 1.1;
			chunked_transfer_encoding off;
			proxy_read_timeout 1w;
		}
	}


# Monitoring

Ding exposes Prometheus metrics at HTTP endpoint /metrics.
This includes statistics on usage for the API.

You can also set up simple HTTP monitoring on /ding/status. It's
the "status" API call and it will raise a 5xx status when one of
its underlying services (file system, database) is not available.


# Service file

Example service file for systemd:

	[Unit]
	Description=ding
	After=network.target

	[Service]
	UMask=0027
	Restart=always
	RestartSec=1s
	LimitNOFILE=16384
	SyslogIdentifier=ding
	SyslogFacility=local0
	User=root
	Group=root
	WorkingDirectory=/home/irias/projects/ding
	ExecStart=/home/irias/projects/ding/ding serve -listen 127.0.0.1:6084 -listenwebhook 127.0.0.1:6085 config.json

	[Install]
	WantedBy=multi-user.target

This listens only on the loopback IP. Note we don't keep the binary
and config in the (mostly empty) ding home directory, /home/ding.
