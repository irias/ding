# coding=utf-8
# VERSION: 0.0.3

import sys, os, subprocess, re, json, md5, shutil

def make_target_fn(dest):
	def target(path):
		return dest+'/'+path
	return target

def ensuredir(path):
	try:
		os.makedirs(os.path.dirname(path))
	except Exception:
		pass
	return True

def concat(l):
	return ''.join([open(e, 'rb').read() for e in l])

def write(path, contents):
	ensuredir(path)
	try:
		if False and open(path, 'rb').read() == contents:
			return
	except EnvironmentError:
		pass
	print >>sys.stderr, 'write', path
	open(path, 'wb').write(contents)
	return True

def jshint(*l):
	jshintpath = os.path.join('node_modules', '.bin', 'jshint')
	run(jshintpath, *l)
	return True

def read(path):
	return open(path, 'rb').read()

def copy(dest, *srcs):
	srcs = list(srcs)
	write(dest, concat(srcs))
	return True

def makehash(path):
	try:
		return md5.md5(open(path, 'rb').read()).hexdigest()[:12]
	except Exception:
		return ''

def revrepl(contents, dest):
	l = re.split('\\s(href|src)="([^"]+)\\?v=([a-zA-Z0-9]+)"', contents)
	r = ''
	while l:
		r += l.pop(0)
		if not l:
			break
		r += ' '+l.pop(0)+'="'
		path = l.pop(0)
		v = l.pop(0)
		v = makehash(make_target_fn(dest)(path))
		if v:
			path += '?v=%s' % v
		r += path
		r += '"'
	return r

def run(*args, **kwargs):
	print >>sys.stderr, 'run', ' '.join(args)
	if os.name == 'nt':
		kwargs['shell'] = True
	subprocess.check_call(args, **kwargs)
	return True

def dirlist(path, suffix='', prefix=''):
	return ['%s/%s' % (path, f) for f in os.listdir('./'+path) if f.endswith(suffix) and f.startswith(prefix)]

def dirtree(path, suffix=''):
	files = []
	for subpath, subdirs, subfiles in os.walk(path):
		for filename in subfiles:
			if filename.endswith(suffix):
				file = '%s/%s' % (subpath, filename)
				file = file.replace('\\', '/')
				files.append(file)
	return files

def ngtemplates(modname, paths, strip_prefix, add_prefix):
	r = 'angular.module(%s, []).run(["$templateCache", function($templateCache) {\n' % json.dumps(modname)
	for path in paths:
		html_path = path
		if not html_path.startswith(strip_prefix):
			raise Exception('unexpected files for template: %r' % html_path)
		html_path = html_path[len(strip_prefix):]
		html_path = add_prefix+html_path
		r += '$templateCache.put(%s,%s);\n' % (json.dumps(html_path), json.dumps(read('./'+path)))
	r += '}]);\n'
	return r

def test(d, s, fn):
	if not isinstance(s, list):
		raise Exception('test: "s" must be a list')

	try:
		dmtime = os.stat('./'+d).st_mtime
	except EnvironmentError as e:
		dmtime = 0

	try:
		smtime = max([os.stat('./'+e).st_mtime for e in s])
	except EnvironmentError as e:
		fn()
		return

	if smtime > dmtime:
		fn()
	return True

def test_copy(d, *sl):
	sl = list(sl)
	return test(d, sl, lambda: copy(d, *sl))

def remove_tree(path):
	try:
		print >>sys.stderr, 'removing tree', path
		shutil.rmtree(path)
	except EnvironmentError:
		pass # doesn't exist, likely
	return True

def remove_file(path):
	try:
		print >>sys.stderr, 'removing file', path
		os.unlink(path)
	except EnvironmentError:
		pass # doesn't exist, likely
	return True
