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
		+ bl.dirlist(srcdir + 'js/ctlrs', suffix='.js') \
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
