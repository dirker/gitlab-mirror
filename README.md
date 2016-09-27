Easy read-only mirroring of gitlab repositories.
================================================

Prerequisites:
--------------
  * git >=2.9 (maybe less, but this is what's tested)

Installation:
-------------
  * adduser git --disabled-password
  * as git user:
    * `ssh-keygen`, no passphrase, add pubkey to gitlab admin user
    * `ssh -T <gitlab-instance>`, accept host key
    * create gitlab-mirror.conf
      * use account token of user the pubkey was added to
    * `gitlab-mirror sync`
    * add cronjobs calling `gitlab-mirror sync` regularly

Configuration:
--------------

Self-explanatory? Fixme!

```
gitlab_token = "<admin-token>"
gitlab = "https://your.gitlab-instace.com"
repository_path = "/where/to/store/your/mirrored/repos"
repos = [
  "list/",
  "of/",
  "prefixes/",
  "of/projects",
  "to/mirror"
]
```

TODO:
-----
  * prevent from running multi sync* commands at once (using lockfile)
  * fix FIXME's in the code
  * allow admins to run gitlab-mirror specific commands via ssh
  * restructure code to be nice
  * handle project removal
  * remove relying on cronjobs, be a daemon instead?
    * sync via timers/goroutines
    * allow to receive system webhooks, sync on push/tag events
