# Installing

You'll need an empty postgres database, and a config.json file like:

	{
		"showSherpaErrors": true,
		"printSherpaErrorStack": true,
		"database": "dbname=ding host=localhost user=ding password=secretpassword sslmode=disable",
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
		"isolateBuilds": {
			"enabled": false,
			"dingUid": 1001,
			"dingGid": 1001,
			"uidStart": 10000,
			"uidEnd": 20000,
			"chownBuild": [
				"sudo", "/home/ding/ding", "chownbuild", "/home/ding/config.json"
			],
			"runas": [
				"/home/ding/runas"
			],
			"buildsDir": "/home/ding/data/build"
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


# Notifications

You probably want to enable email notifications for failed builds.
Configure a mail server, and set "mail", "enabled" to true.


# Isolate builds

You should also isolate builds by running each build under a unique user id.
You'll need some more configuration.
First, configure sudo to run the "ding chownbuild" command that is
already specified in the config file:

	# template: executing-user (hosts) = (user-to-run-as) dont-ask-for-password path-with-parameters
	ding ALL = (ALL) NOPASSWD:/home/ding/ding chownbuild /home/ding/config.json *

Second, create a command /home/ding/runas. It will be called as
"runas uid gid command param ...". It must run the command with
params under the specified uid/gid. "uidStart" and "uidEnd"
in the config file specify a range of uids that will be used.
Unfortunately, sudo isn't usable in this case, you cannot specify
uid ranges. Instead, use the runas.c file from the ding repository:

	cd /home/ding
	cc -Wall -o runas.c runas
	chown root:ding runas
	chmod 4750 runas
	cat 10000 >/etc/runas.conf
	chown root:ding /etc/runas.conf
	chmod 640 /etc/runas.conf

Set "uid" and "gid" to the uid/gid ding is running under.
Finally, set "isolateBuilds" to true.


# Github webhooks for push events

Ding supports starting builds on pushes to github repositories.
Start ding with the -githublisten flag and set "githubWebhookSecret"
in the config file. You'll need to configure a "webhook" for your
repositories at github.com, select "application/json" as event type,
send only "push" events (default at the time of writing), and set
the same secret as in the config file.

If you don't want to listen for github webhook events, pass an empty
string to the -githublisten flag.
