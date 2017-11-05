/*
 * simple tool to run a command as another user, intended to be installed setuid root, executable by the gid that is allowed to start builds. E.g. to run as uid 10123, gid 1038, with environment left intact:
 *
 *      /path/to/runas 10123 1038 ./build.sh
 *
 * The minimum uid is read from /etc/runas.conf, which must contain just a single number.
 * Any gid except 0 is allowed.
 */

#include <sys/types.h>
#include <sys/errno.h>
#include <stdio.h>
#include <unistd.h>
#include <stdlib.h>
#include <err.h>
#include <fcntl.h>
#include <string.h>

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
	if(setgid(gid) != 0) {
		err(1, "setgid");
	}
	if(setuid(uid) != 0) {
		err(1, "setuid");
	}
	execvp(argv[3], argv+3);
	err(1, "execvp");
}
