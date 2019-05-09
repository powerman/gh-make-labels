package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/google/go-github/github"
	"github.com/powerman/structlog"
	"gopkg.in/yaml.v2"
)

//nolint:gochecknoglobals
var (
	cmd = strings.TrimSuffix(path.Base(os.Args[0]), ".test")
	ver string // set by ./build
	log = structlog.NewZeroLogger().
		SetPrefixKeys(structlog.KeyLevel).
		SetKeysFormat(map[string]string{
			structlog.KeyLevel:   "%[2]s",
			structlog.KeyMessage: " %#[2]q",
			"err":                " %s: %s",
		})
	cfg struct { //nolint:maligned
		version    bool
		logLevel   string
		configPath string
		cleanup    bool
		owner      string
		repo       string
	}
)

func main() {
	flag.StringVar(&cfg.logLevel, "log.level", "info", "log `level` (debug|info|warn|err)")
	flag.StringVar(&cfg.configPath, "config", "gh-labels.yml", "`path` to config file")
	flag.BoolVar(&cfg.cleanup, "cleanup", false, "delete unknown labels")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] owner/repo\n", cmd)
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 1 && strings.Count(flag.Arg(0), "/") == 1 {
		ownerRepo := strings.Split(flag.Arg(0), "/")
		cfg.owner, cfg.repo = ownerRepo[0], ownerRepo[1]
	}

	switch {
	case cfg.owner == "" || cfg.repo == "":
		flag.Usage()
		os.Exit(2)
	case cfg.version: // Must be checked after all other flags for ease testing.
		fmt.Println(cmd, ver, runtime.Version())
		os.Exit(0)
	}

	// Wrong log.level is not fatal, it will be reported and set to "debug".
	log.SetLogLevel(structlog.ParseLevel(cfg.logLevel))

	labels, err := loadLabelsCfg(cfg.configPath)
	if err != nil {
		log.Fatal(err)
	}

	user, token, err := loadHubCfg()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	err = makeGitHubLabels(ctx, user, token, cfg.owner, cfg.repo, labels, cfg.cleanup)
	if err != nil {
		os.Exit(1)
	}
}

func loadLabelsCfg(configPath string) (map[string]string, error) {
	buf, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var labelCfg struct {
		Labels map[string]string
		Colors map[string]string
	}
	err = yaml.Unmarshal(buf, &labelCfg)
	if err != nil {
		return nil, err
	}

	labels := make(map[string]string, len(labelCfg.Labels))
	for label, colorName := range labelCfg.Labels {
		colorHex, ok := labelCfg.Colors[colorName]
		if !ok {
			return nil, fmt.Errorf("label %q: missing color %q", label, colorName)
		}
		labels[label] = colorHex
	}
	return labels, nil
}

func loadHubCfg() (user, token string, err error) {
	buf, err := ioutil.ReadFile(os.Getenv("HOME") + "/.config/hub")
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", errors.New("hub tool is not installed or not configured")
		}
		return "", "", err
	}

	var hubCfg = make(map[string][]struct {
		Protocol   string
		User       string
		OAuthToken string `yaml:"oauth_token"`
	})
	err = yaml.Unmarshal(buf, &hubCfg)
	if err != nil {
		return "", "", err
	}
	if len(hubCfg["github.com"]) == 0 {
		return "", "", errors.New("failed to detect configuration of hub tool")
	}

	return hubCfg["github.com"][0].User, hubCfg["github.com"][0].OAuthToken, nil
}

func makeGitHubLabels( //nolint:gocyclo
	ctx context.Context,
	user string,
	pass string,
	owner string,
	repo string,
	newLabels map[string]string,
	cleanup bool,
) error {
	auth := &github.BasicAuthTransport{Username: user, Password: pass}
	client := github.NewClient(auth.Client())

	var oldLabels []*github.Label
	const perPage = 100
	for page := 1; ; page++ {
		opt := &github.ListOptions{Page: page, PerPage: perPage}
		labels, _, err := client.Issues.ListLabels(ctx, owner, repo, opt)
		if err != nil {
			return log.Err("failed to list labels", "err", prettify(err))
		}
		oldLabels = append(oldLabels, labels...)
		if len(labels) < perPage {
			break
		}
	}

	var err error
	for _, l := range oldLabels {
		label, color := *l.Name, *l.Color
		switch {
		case newLabels[label] == "":
			if !cleanup {
				log.Debug("ignore", "label", label)
			} else {
				_, err = client.Issues.DeleteLabel(ctx, owner, repo, label)
				if err != nil {
					log.Warn("failed to remove", "label", label, "err", prettify(err))
				} else {
					log.Info("remove", "label", label)
				}
			}
		case newLabels[label] != color:
			*l.Color = newLabels[label]
			_, _, err = client.Issues.EditLabel(ctx, owner, repo, label, l)
			if err != nil {
				log.Warn("failed to update", "label", label, "color", newLabels[label], "err", prettify(err))
			} else {
				log.Info("update", "label", label, "color", newLabels[label], "old_color", color)
			}
		default:
			log.Debug("exists", "label", label)
		}
		delete(newLabels, label)
	}

	for label, color := range newLabels {
		l := &github.Label{Name: github.String(label), Color: github.String(color)}
		_, _, err = client.Issues.CreateLabel(ctx, owner, repo, l)
		if err != nil {
			log.Warn("failed to create", "label", label, "color", color, "err", prettify(err))
		} else {
			log.Info("create", "label", label, "color", color)
		}
	}

	return err
}

func prettify(err error) error {
	switch err := err.(type) {
	default:
		return err
	case *github.ErrorResponse:
		switch {
		case len(err.Errors) != 1:
			return err
		case err.Errors[0].Code == "custom":
			return errors.New(err.Errors[0].Message)
		default:
			return errors.New(err.Errors[0].Code)
		}
	}
}
