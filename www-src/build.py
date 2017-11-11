#!/usr/bin/env python
# coding=utf-8

import sys, os, os.path, subprocess, re, json, md5, shutil
import buildlib as bl

destination = 'assets'
srcdir = os.path.split(os.path.dirname(sys.argv[0]))[1] + '/'  # figure out the path-relative directory name where this script resides

def build(dest):
	target = bl.make_target_fn(dest)

	# angularjs templates
	d = target('static/js/app-templates.js')
	s = bl.dirtree(srcdir + 'html', '.html')
	bl.test(d, s, lambda: bl.write(d, bl.ngtemplates('templates', s, srcdir + 'html/', 'static/html/')))


	# images
	s = bl.dirlist(srcdir + 'img')
	for e in s:
		d = target('static/img/%s' % os.path.basename(e))
		bl.test(d, [e], lambda: bl.copy(d, e))


	# app js
	d = target('static/js/app.js')
	s = [
		srcdir + 'js/app.js',
		srcdir + 'js/app.config.js',
		] \
		+ bl.dirlist(srcdir + 'js/ctlr', suffix='.js') \
		+ bl.dirlist(srcdir + 'js/directives', suffix='.js') \
		+ bl.dirlist(srcdir + 'js/filters', suffix='.js') \
		+ bl.dirlist(srcdir + 'js/services', suffix='.js')
	bl.test(d, s, lambda: bl.jshint(*s) and bl.copy(d, *s))


	# vendor js
	d = target('static/js/app-vendor.js')
	s = [
		srcdir + 'vendors/js/jquery-3.1.0.min.js',
		srcdir + 'vendors/js/angular-1.5.7.min.js',
		srcdir + 'vendors/js/angular-route-1.5.7.min.js',
		srcdir + 'vendors/js/ui-bootstrap-tpls-1.3.3.min.js',
		srcdir + 'vendors/js/lodash-4.13.1.min.js',
	]
	bl.test(d, s, lambda: bl.copy(d, *s))


	# vendor css
	d = target('static/css/app-vendor.css')
	s = [
		srcdir + 'vendors/bootstrap-3.3.6/css/bootstrap.min.css',
		srcdir + 'vendors/font-awesome-4.6.3/css/font-awesome.css',
	]
	bl.test(d, s, lambda: bl.copy(d, *s))


	# app css
	d = target('static/css/app.css')
	s = srcdir + 'scss/app.scss'
	bl.test(d, [s]+bl.dirlist(srcdir + 'scss', suffix='.scss', prefix='_'), lambda: bl.ensuredir(d) and bl.run('sass', '--style', 'compact', s, d))


	# fonts
	s = bl.dirlist(srcdir + 'vendors/font-awesome-4.6.3/fonts') + bl.dirlist(srcdir + 'vendors/bootstrap-3.3.6/fonts')
	for e in s:
		d = target('static/fonts/%s' % os.path.basename(e))
		bl.test(d, [e], lambda: bl.copy(d, e))


	# index.html
	d = target('index.html')
	s = [
		srcdir + 'index.html',
		target('static/css/app-vendor.css'),
		target('static/css/app.css'),
		target('static/js/app-vendor.js'),
		target('static/js/app-templates.js'),
		target('static/js/app.js'),
		target('static/img/logo.png'),
	]
	bl.test(d, s, lambda: bl.write(d, bl.revrepl(bl.read(srcdir + 'index.html'), dest)))

	files = [
		'favicon.ico',
		'robots.txt',
	]
	for name in files:
		d = target(name)
		s = srcdir + name
		bl.test(d, [s], lambda: bl.copy(d, s))

	s = 'INSTALL.md'
	d = target(s)
	bl.test(d, [s], lambda: bl.copy(d, s))


	d = target('LICENSES')
	s = [
		['Go runtime and standard library',
			['www-src/licenses/go']],
		['Bootstrap 3.3.6',
			['www-src/licenses/bootstrap-3.3.6']],
		['Fontawesome 4.6.3\n\nFont Awesome by Dave Gandy - http://fontawesome.io\nFont licensed under SIL OFL 1.1-license\nCode, such as CSS, under MIT-license',
			[]],
		['jQuery 3.1.0',
			['www-src/licenses/jquery-3.1.0']],
		['lodash 4.13.1',
			['www-src/licenses/lodash-4.13.1']],
		['AngularJS including the route module 1.5.7',
			['www-src/licenses/angularjs-1.5.7']],
		['UI Bootstrap 1.3.3',
			['www-src/licenses/ui-bootstrap-1.3.3']],
		['Sherpa Go server library',
			['vendor/bitbucket.org/mjl/sherpa/LICENSE']],
		['httpasset Go library',
			['vendor/bitbucket.org/mjl/httpasset/LICENSE']],
		['', ['vendor/github.com/beorn7/perks/LICENSE']],
		['', ['vendor/github.com/golang/protobuf/LICENSE']],
		['', ['vendor/github.com/irias/sherpa-prometheus-collector/LICENSE.md']],
		['', ['vendor/github.com/lib/pq/LICENSE.md']],
		['', ['vendor/github.com/matttproud/golang_protobuf_extensions/LICENSE']],
		['Prometheus Go client', [
			'vendor/github.com/prometheus/client_golang/LICENSE',
			'vendor/github.com/prometheus/client_golang/NOTICE',
			'vendor/github.com/prometheus/client_model/NOTICE',
			'vendor/github.com/prometheus/common/NOTICE',
			'vendor/github.com/prometheus/procfs/NOTICE']],
	]
	ss = []
	for t in s:
		ss += t[1]
	def make_licenses(s):
		r = '# Licenses for software included in Ding\n\n'
		for name, files in s:
			if name == '':
				name = files[0]
				if name.startswith('vendor/'):
					name = name[len('vendor/'):]
			r += '## ' + name + '\n\n'
			for file in files:
				r += open(file).read().decode('utf-8') + '\n'
			r += '\n\n'
		return r.encode('utf-8')
	bl.test(d, ss, lambda: bl.write(d, make_licenses(s)))



	sql = []
	for f in sorted(os.listdir('sql')):
		if not re.search('[0-9]{3}-.*\\.sql', f):
			continue
		versionstr = f.split('-')[0]
		if versionstr == '000':
			version = 0
		else:
			version = int(versionstr, 10)
		sql.append(dict(version=version, filename=f))
	def sql_json():
		for e in sql:
			e['sql'] = open('sql/' + e['filename']).read()
		return json.dumps(sql)
	d = target('sql.json')
	bl.test(d, ['sql'] + ['sql/' + e['filename'] for e in sql], lambda: bl.write(d, sql_json()))
	

def usage():
	print >>sys.stderr, 'usage: build.py [clean | install | frontend] ...'
	sys.exit(1)

def main(prog, *args):
	args = list(args)
	if args == []:
		args = ['install']
	for t in args:
		if len(args) > 1:
			print >>sys.stderr, '# %s:' % t

		if t == 'clean':
			bl.remove_tree(destination)

		elif t == 'install':
			build(destination)

		else:
			usage()

if __name__ == '__main__':
	main(*sys.argv)
