Tarsnap Manager
===============

* Creates Tarsnap archives in standard formats - nightly and adhoc.
* Expires backups after time specified in the configuration file.

Configuration Format
--------------------

Configuration is a simplified YAML as provided by [Gypsy](https://github.com/kylelemons/go-gypsy).

Options:

* **TarsnapBin** (string)

  The path to the tarsnap binary. Defaults to /usr/local/bin/tarsnap.

* **TarsnapArgs** (list)

  Arguments to be passed to tarsnap. Parameters (starting with a dash) must be
  quoted in double-quotes. Quotes will be removed prior to execution.

* **BackupDirs** (list)

  Directories to be backed up.

* **KeepWeeks** (int)

  The number of weeks to retain nightly backups.

* **KeepMonths** (int)

  The number of months to retain monthly backups.

* **ExpireBackups** (bool)

  Whether to enable expiring of backups. Disabled by default.

Example
-------

```
TarsnapBin: /usr/bin/tarsnap
TarsnapArgs:
 - "-X excludefile"
BackupDirs:
 - /etc
 - /var/log
KeepWeeks: 5
KeepMonths: 24
ExpireBackups: true
```
