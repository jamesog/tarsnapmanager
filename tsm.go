package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/kylelemons/go-gypsy/yaml"
)

// Default configuration file
var conffile = flag.String("c", ".tsmrc", "config file")
var showAllBackups = flag.Bool("with-current", false, "list-expired: list current backups too")

// Config variables. Provide a default for tarsnap(1) path.
var cfgTarsnapBin = "/usr/local/bin/tarsnap"
var cfgTarsnapArgs []string
var cfgBackupDirs []string
var cfgExcludeFile string

// Templates for time.Parse()
const iso8601 = "2006-01-02"
const nightly = "nightly-2006-01-02"
const adhoc = "adhoc-2006-01-02_1504"

const day = time.Hour * 24

var info = log.New(os.Stdout, "", log.LstdFlags)

// Shamefully "borrowed" from src/cmd/go/main.go
// Flattens a mix of strings and slices of strings into a single slice.
func commandArgs(args ...interface{}) []string {
	var x []string
	for _, arg := range args {
		switch arg := arg.(type) {
		case []string:
			x = append(x, arg...)
		case string:
			x = append(x, arg)
		default:
			panic("commandArgs: invalid argument")
		}
	}
	return x
}

// Creates a new Tarsnap archive
func runBackup(archiveName string) {
	info.Printf("Starting backup %s\n", archiveName)
	args := commandArgs("-c", "-f", archiveName, cfgTarsnapArgs, cfgBackupDirs)
	backup := exec.Command(cfgTarsnapBin, args...)
	backup.Stdout = os.Stdout
	backup.Stderr = os.Stderr
	backuperr := backup.Run()
	if backuperr != nil {
		log.Fatal("Error running backup: ", backuperr)
	}
	info.Println("Backup finished")
}

// Deletes a Tarsnap archive
func deleteBackup(backup string) {
	deletecmd := exec.Command(cfgTarsnapBin, "-d", "-f", backup)
	deletecmd.Stdout = os.Stdout
	deletecmd.Stderr = os.Stderr
	err := deletecmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

// Runs expiry against backup archives
func expireBackups(w, m time.Time, reallyExpire bool) {
	listcmd := exec.Command(cfgTarsnapBin, "--list-archives")
	var stdout bytes.Buffer
	listcmd.Stdout = &stdout
	listcmd.Stderr = os.Stderr
	listerr := listcmd.Run()
	if listerr != nil {
		log.Fatal("Error running command: ", listerr)
	}
	backups := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	sort.Strings(backups)

	for i := 0; i < len(backups); i++ {
		// Don't expire adhoc backups
		if strings.HasPrefix(backups[i], "adhoc-") {
			continue
		}
		backup, _ := time.Parse(nightly, backups[i])
		eom := time.Date(backup.Year(), backup.Month()+1, 0, 0, 0, 0, 0, backup.Location())
		if (backup.Before(w) && backup.Day() != eom.Day()) || backup.Before(m) {
			if reallyExpire {
				info.Println("Expiring backup", backups[i])
				deleteBackup(backups[i])
			} else {
				fmt.Println("Expired backup", backups[i])
			}
		} else {
			if *showAllBackups && !reallyExpire {
				fmt.Println("Current backup", backups[i])
			}
		}
	}
}

func main() {
	flag.Parse()

	config, conferr := yaml.ReadFile(*conffile)
	if conferr != nil {
		log.Fatalf("Error reading config %q: %s", *conffile, conferr)
	}

	tmpTarsnapBin, _ := config.Get("TarsnapBin")
	if tmpTarsnapBin != "" {
		cfgTarsnapBin = tmpTarsnapBin
	}

	count, err := config.Count("TarsnapArgs")
	for i := 0; i < count; i++ {
		s := fmt.Sprintf("TarsnapArgs[%d]", i)
		t, err := config.Get(s)
		if err != nil {
			log.Fatal(err)
		}
		// Remove any quotes from the arg - used to protect
		// options (starting with a -)
		t = strings.Replace(t, `"`, ``, -1)
		cfgTarsnapArgs = append(cfgTarsnapArgs, t)
	}

	count, err = config.Count("BackupDirs")
	if err != nil {
		fmt.Println("No backup directories specified")
		os.Exit(1)
	}
	for i := 0; i < count; i++ {
		s := fmt.Sprintf("BackupDirs[%d]", i)
		t, err := config.Get(s)
		if err != nil {
			log.Fatal(err)
		}
		cfgBackupDirs = append(cfgBackupDirs, t)
	}

	cfgExcludeFile, err := config.Get("ExcludeFile")
	if err == nil {
		cfgTarsnapArgs = append(cfgTarsnapArgs, "-X", cfgExcludeFile)
	}

	// GetInt() returns an int64. Convert this to an int.
	tmpKeepWeeks, err := config.GetInt("KeepWeeks")
	if err != nil {
		fmt.Println("Missing config value KeepWeeks")
		os.Exit(1)
	}
	tmpKeepMonths, err := config.GetInt("KeepMonths")
	if err != nil {
		fmt.Println("Missing config value KeepMonths")
		os.Exit(1)
	}
	cfgKeepWeeks := int(tmpKeepWeeks)
	cfgKeepMonths := int(tmpKeepMonths)

	t := time.Now()
	w := t.AddDate(0, 0, -(7 * cfgKeepWeeks))
	m := t.AddDate(0, -cfgKeepMonths, 0)
	fmt.Printf("Date: %s\nExpire week: %s\nExpire month: %s\n\n", t.Format(iso8601), w.Format(iso8601), m.Format(iso8601))

	if len(flag.Args()) == 0 {
		fmt.Println("Missing action")
		os.Exit(1)
	}
	action := flag.Args()[0]
	switch action {
	case "nightly":
		// Run nightly
		runBackup(t.Format(nightly))

		// TODO: Make w and m global?
		cfgExpireBackups, _ := config.GetBool("ExpireBackups")
		if cfgExpireBackups {
			expireBackups(w, m, true)
		} else {
			info.Println("Backup expiration disabled")
		}

		info.Println("All done!")
	case "adhoc":
		// Run adhoc
		runBackup(t.Format(adhoc))
	case "list-expired":
		expireBackups(w, m, false)
	default:
		log.Fatalf("Unknown action '%s'", action)
	}
}
