/*
Simple tool to run a command as another user, intended to be installed setuid root, executable by the gid that is allowed to start builds. E.g. to run as uid 10123, gid 1038, with environment left intact:

	/path/to/runas 10123 1038 ./build.sh

The minimum uid is read from /etc/runas.conf, which must contain just a single number.
Any gid except 0 is allowed.

Compile:

	cc -Wall -DWITH_SETRESUID -o runas runas.c  # for openbsd
	cc -Wall -DWITH_SETRESUID -D_GNU_SOURCE -o runas runas.c  # for linux
	cc -Wall -o runas runas.c  # for macos
 */

#include <sys/types.h>
#include <sys/errno.h>
#include <sys/param.h>
#include <stdio.h>
#include <unistd.h>
#include <stdlib.h>
#include <err.h>
#include <fcntl.h>
#include <string.h>
#include <grp.h>

typedef unsigned int uint;

void
usage(void) {
	fprintf(stderr, "usage: runas uid gid command\n");
	exit(2);
}

void
readstr(int fd, char *sbuf, char *ebuf) {
	for(;;) {
		if (sbuf == ebuf) {
			errx(1, "file too large");
		}
		int n = read(fd, sbuf, ebuf-sbuf);
		if (n < 0) {
			err(1, "reading string");
		}
		if (n == 0) {
			*sbuf = '\0';
			return;
		}
		sbuf += n;
	}
}

uint
parseint(char *s) {
	char *end;
	errno = 0;
	uint uid = strtol(s, &end, 10);
	if(errno != 0) {
		err(1, "invalid uid");
	}
	if(end == s) {
		errx(1, "invalid uid, empty input");
	}
	if(*end != '\0') {
		errx(1, "invalid uid, leftover data after number");
	}
	return uid;
}

int
main(int argc, char *argv[]) {
	if (sizeof(uint) > sizeof(uid_t)) {
		errx(1, "sizeof uid_t is smaller than sizeof uint");
	}

	if(argc < 4) {
		usage();
	}

	int fd = open("/etc/runas.conf", O_RDONLY);
	if (fd < 0) {
		err(1, "open config file");
	}
	char buf[128];
	readstr(fd, buf, buf + sizeof buf);
	int n = strlen(buf);
	if (n > 0 && buf[n-1] == '\n') {
		buf[n-1] = '\0';
	}
	close(fd);

	uint base = parseint(buf);
	uint uid = parseint(argv[1]);
	uint gid = parseint(argv[2]);
	if(uid < base) {
		errx(1, "uid not allowed, but be >= base from config file");
	}
	if(gid == 0) {
		errx(1, "gid 0 is not allowed");
	}

#ifdef WITH_SETRESUID
	if(setresgid(gid, gid, gid) != 0) {
		err(1, "setresgid");
	}
	uid_t rgid, egid, sgid;
	if(getresgid(&rgid, &egid, &sgid) != 0) {
		err(1, "getresgid");
	}
	if(rgid != gid || egid != gid || sgid != gid) {
		errx(1, "not all gids were correct after setresgid");
	}

	if(setgroups(1, &gid) != 0) {
		err(1, "setgroups");
	}

	if(setresuid(uid, uid, uid) != 0) {
		err(1, "setresuid");
	}
	uid_t ruid, euid, suid;
	if(getresuid(&ruid, &euid, &suid) != 0) {
		err(1, "getresuid");
	}
	if(ruid != uid || euid != uid || suid != uid) {
		errx(1, "not all uids were correct after setresuid");
	}
#else
	if(geteuid() != 0) {
		err(1, "must be called with effective uid 0");
	}

	if (setregid(gid, gid) != 0) {
		err(1, "setregid");
	}
	if(getgid() != gid || getegid() != gid) {
		err(1, "real or effective gid not as expected");
	}
	// if only there was a way to check sgid...

	if(setgroups(1, &gid) != 0) {
		err(1, "setgroups");
	}

	if(setreuid(uid, uid) != 0) {
		err(1, "setreuid");
	}
	if(getuid() != uid || geteuid() != uid) {
		errx(1, "real or effective uid not as expected");
	}
	/// if only there was a way to check suid...
#endif

	execvp(argv[3], argv+3);
	err(1, "execvp");
}
