package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/kylelemons/go-gypsy/yaml"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Default configuration file
var conffile = flag.String("c", ".tsmrc", "config file")

// Config variables. Provide a default for tarsnap(1) path.
var cfgTarsnapBin string = "/usr/local/bin/tarsnap"
var cfgTarsnapArgs []string
var cfgBackupDirs []string

// Templates for time.Parse()
const iso8601 = "2006-01-02"
const nightly = "nightly-2006-01-02"
const adhoc = "adhoc-2006-01-02_1504"

const day = time.Hour * 24

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
	log.Printf("Starting backup %s\n", archiveName)
	args := commandArgs("-c", "-f", archiveName, cfgTarsnapArgs, cfgBackupDirs)
	backup := exec.Command(cfgTarsnapBin, args...)
	var stderr bytes.Buffer
	backup.Stderr = &stderr
	backuperr := backup.Run()
	if backuperr != nil {
		log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)
		log.Println(stderr.String())
		log.Fatal(backuperr)
	}
	log.Println("Backup finished")
}

// Deletes a Tarsnap archive
func deleteBackup(backup string) {
	deletecmd := exec.Command(cfgTarsnapBin, "-d", "-f", backup)
	err := deletecmd.Run()
	if err != nil {
		log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)
		log.Fatal(err)
	}
}

// Runs expiry against backup archives
func expireBackups(w, m time.Time) {
	listcmd := exec.Command(cfgTarsnapBin, "--list-archives")
	var out bytes.Buffer
	listcmd.Stdout = &out
	listerr := listcmd.Run()
	if listerr != nil {
		log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)
		log.Fatal(listerr)
	}
	backups := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	sort.Strings(backups)

	for i := 0; i < len(backups); i++ {
		backup, _ := time.Parse(nightly, backups[i])
		eom := time.Date(backup.Year(), backup.Month()+1, 0, 0, 0, 0, 0, backup.Location())
		if (backup.Before(w) && backup.Day() != eom.Day()) || backup.Before(m) {
			log.Println("Expiring backup", backups[i])
			deleteBackup(backups[i])
		} else {
			log.Println("Keeping backup", backups[i])
		}
	}
}

func main() {
	flag.Parse()

	config, conferr := yaml.ReadFile(*conffile)
	if conferr != nil {
		log.Fatalf("Read config %q: %s", *conffile, conferr)
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
		log.Fatal("No backup directories specified")
	}
	for i := 0; i < count; i++ {
		s := fmt.Sprintf("BackupDirs[%d]", i)
		t, err := config.Get(s)
		if err != nil {
			log.Fatal(err)
		}
		cfgBackupDirs = append(cfgBackupDirs, t)
	}

	// GetInt() returns an int64. Convert this to an int.
	tmpKeepWeeks, err := config.GetInt("KeepWeeks")
	if err != nil {
		log.Fatal("Missing config value KeepWeeks")
	}
	tmpKeepMonths, err := config.GetInt("KeepMonths")
	if err != nil {
		log.Fatal("Missing config value KeepMonths")
	}
	cfgKeepWeeks := int(tmpKeepWeeks)
	cfgKeepMonths := int(tmpKeepMonths)

	t := time.Now()
	w := t.AddDate(0, 0, -(7 * cfgKeepWeeks))
	m := t.AddDate(0, -cfgKeepMonths, 0)
	fmt.Printf("Date: %s\nExpire week: %s\nExpire month: %s\n", t.Format(iso8601), w.Format(iso8601), m.Format(iso8601))
	fmt.Println()

	if len(os.Args) < 2 {
		log.Fatal("Missing action")
	}
	action := os.Args[1]
	switch action {
	case "nightly":
		// Run nightly
		runBackup(t.Format(nightly))

		// TODO: Make w and m global?
		cfgExpireBackups, _ := config.GetBool("ExpireBackups")
		if cfgExpireBackups {
			expireBackups(w, m)
		} else {
			log.Println("Backup expiration disabled")
		}
	case "adhoc":
		// Run adhoc
		runBackup(t.Format(adhoc))
	default:
		log.Fatalf("Unknown action '%s'", action)
	}

	log.Println("All done!")
}
