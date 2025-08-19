// A simple tool to manage GitHub labels based on a YAML configuration file.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/google/go-github/github"
	"github.com/powerman/structlog"
	"gopkg.in/yaml.v2"
)

const exitCodeBadArgs = 2

var (
	errMissingColor = errors.New("missing color")
	errNoGHConfig   = errors.New("failed to detect configuration of gh tool")
	errNoGH         = errors.New("gh tool is not installed or not configured")
)

//nolint:gochecknoglobals // By design.
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
	cfg struct {
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
	flag.Usage = func() { //nolint:reassign // By design.
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
		os.Exit(exitCodeBadArgs)
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

	user, token, err := loadGHCfg()
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
	buf, err := os.ReadFile(configPath) //nolint:gosec // False positive.
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
			return nil, fmt.Errorf("label %q: %w %q", label, errMissingColor, colorName)
		}
		labels[label] = colorHex
	}
	return labels, nil
}

func loadGHCfg() (user, token string, err error) {
	buf, err := os.ReadFile(os.Getenv("HOME") + "/.config/gh/hosts.yml")
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", errNoGH
		}
		return "", "", err
	}

	ghCfg := make(map[string]struct {
		Protocol   string
		User       string
		OAuthToken string `yaml:"oauth_token"`
	})
	err = yaml.Unmarshal(buf, &ghCfg)
	if err != nil {
		return "", "", err
	}
	if _, ok := ghCfg["github.com"]; !ok {
		return "", "", errNoGHConfig
	}

	return ghCfg["github.com"].User, ghCfg["github.com"].OAuthToken, nil
}

func makeGitHubLabels( //nolint:gocyclo,funlen,gocognit,revive // TODO: Refactor.
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
	var errResp *github.ErrorResponse
	switch {
	case errors.As(err, &errResp):
		switch {
		case len(errResp.Errors) != 1:
			return errResp
		case errResp.Errors[0].Code == "custom":
			return errors.New(errResp.Errors[0].Message) //nolint:err113 // By design.
		default:
			return errors.New(errResp.Errors[0].Code) //nolint:err113 // By design.
		}
	default:
		return errResp
	}
}
