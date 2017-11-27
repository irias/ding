package main

import (
	"fmt"
)

func _sendMailFailing(repo Repo, build Build, errmsg string) {
	link := fmt.Sprintf("%s/#/repo/%s/build/%d/", config.BaseURL, repo.Name, build.ID)
	subject := fmt.Sprintf("ding: failure: repo %s branch %s failing", repo.Name, build.Branch)
	textMsg := fmt.Sprintf(`Hi!

Your build for branch %s on repo %s is now failing:

	%s

Last output:

	%s
	%s

Please fix, thanks!

Cheers,
Ding
`, build.Branch, repo.Name, link, build.LastLine, errmsg)

	_sendmail(config.Notify.Name, config.Notify.Email, subject, textMsg)
}

func _sendMailFixed(repo Repo, build Build) {
	link := fmt.Sprintf("%s/#/repo/%s/build/%d/", config.BaseURL, repo.Name, build.ID)
	subject := fmt.Sprintf("ding: resolved: repo %s branch %s is building again", repo.Name, build.Branch)
	textMsg := fmt.Sprintf(`Hi!

You fixed the build for branch %s on repo %s:

	%s

You're the bomb, keep it up!

Cheers,
Ding
`, build.Branch, repo.Name, link)

	_sendmail(config.Notify.Name, config.Notify.Email, subject, textMsg)
}
